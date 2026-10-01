// Package whip is the HTTP half of WHIP (RFC 9725) and WHEP: POST an SDP offer to
// the endpoint, get a 201 with the SDP answer and the session's resource URL in
// Location, DELETE that URL to end the session. Both protocols are the same
// exchange; only the direction of the media differs.
//
// Every error this package returns is safe to put in a report: it names the host
// at most, never the path, query string, Location or Authorization value, because
// those are where servers carry stream keys and tokens.
package whip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// Error is a failed exchange, classified so a report can count failures by kind.
type Error struct {
	// Kind is a short stable label: "http_status", "transport", "bad_answer".
	Kind string
	// Status is the HTTP status code when Kind is "http_status".
	Status int
	msg    string
}

func (e *Error) Error() string { return e.msg }

// Session is an established WHIP or WHEP session.
type Session struct {
	Answer string
	// Resource is the absolute session URL from Location, or "" when the server
	// sent none (then the session cannot be torn down by DELETE).
	Resource string
}

// Offer POSTs offer to endpoint and returns the answer. bearer, when non-empty, is
// sent as "Authorization: Bearer …" and never appears in an error.
func Offer(ctx context.Context, c *http.Client, endpoint, offer, bearer string) (*Session, error) {
	u, err := url.Parse(endpoint)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, &Error{Kind: "bad_url", msg: "endpoint is not an http(s) URL"}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(offer))
	if err != nil {
		return nil, &Error{Kind: "bad_url", msg: "cannot build the request"}
	}
	req.Header.Set("Content-Type", "application/sdp")
	req.Header.Set("Accept", "application/sdp")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, &Error{Kind: "transport", msg: "POST to " + u.Host + ": " + Scrub(err)}
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, &Error{Kind: "transport", msg: "reading the answer from " + u.Host + ": " + Scrub(err)}
	}
	// RFC 9725 §4.2 says 201 Created; some servers answer 200, which is accepted —
	// the SDP answer is what matters.
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, &Error{Kind: "http_status", Status: resp.StatusCode,
			msg: fmt.Sprintf("POST to %s: HTTP %d", u.Host, resp.StatusCode)}
	}
	if !strings.Contains(string(body), "v=0") {
		return nil, &Error{Kind: "bad_answer", msg: "the answer from " + u.Host + " is not SDP"}
	}
	s := &Session{Answer: string(body)}
	if loc := resp.Header.Get("Location"); loc != "" {
		if ru, err := u.Parse(loc); err == nil {
			s.Resource = ru.String()
		}
	}
	return s, nil
}

// Delete ends the session. A missing resource URL is not an error: there is
// nothing to delete.
func Delete(ctx context.Context, c *http.Client, s *Session, bearer string) error {
	if s == nil || s.Resource == "" {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, s.Resource, nil)
	if err != nil {
		return &Error{Kind: "bad_url", msg: "cannot build the DELETE request"}
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := c.Do(req)
	if err != nil {
		return &Error{Kind: "transport", msg: "DELETE: " + Scrub(err)}
	}
	_ = resp.Body.Close()
	if resp.StatusCode >= 300 {
		return &Error{Kind: "http_status", Status: resp.StatusCode, msg: fmt.Sprintf("DELETE: HTTP %d", resp.StatusCode)}
	}
	return nil
}

// Scrub returns err's message without the URL that net/http puts in a *url.Error.
func Scrub(err error) string {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timed out"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	return err.Error()
}

// Host returns the host[:port] of a URL and nothing else — the only part of an
// endpoint a report records. An unparseable URL gives "invalid-url".
func Host(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "invalid-url"
	}
	return u.Host
}
