# Changelog

All notable changes to whipbench. The format is [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versions follow [SemVer](https://semver.org/). Items reference their `WB-n` backlog id.

## [Unreleased]

### Security
- golang.org/x/crypto 0.48.0 → 0.52.0 (with x/net 0.54.0, x/sys 0.45.0), after a
  Dependabot security alert on the first push.

### Fixed
- CI lint builds golangci-lint with the module's Go 1.27 instead of using the release
  binary, which is built with Go 1.26 and refuses the module.

## [0.0.1] — 2026-10-01

Not released: the first binary, published as source so the clients, the report and the
operating model can be reviewed before the latency method (WB-1) is decided. No tag.

### Added
- `whipbench publish`: a WHIP publisher streaming an embedded synthetic clip (640×360,
  30 fps, 4 s, 1 s GOP) in VP8 or constrained-baseline H.264, looped with continuous RTP
  timestamps and paced at the frame rate; every packet carries its send time in the
  abs-capture-time header extension when the server accepts it (WB-9).
  `scripts/make-clips.sh` records the ffmpeg command that made the clips.
- `whipbench view`: N concurrent native WHEP viewers recording join time to the first
  RTP packet and to the first complete keyframe, loss from sequence numbers, RFC 3550
  interarrival jitter, keyframe interval, stalls, and one-way delay — or the reason it
  is unavailable (WB-10).
- `whipbench run scenario.json`: one publisher and viewers on a linear ramp, from a
  strict declarative JSON scenario (`viewers`, `rampSeconds`, `holdSeconds`, …) (WB-11).
- Reports in JSON (`whipbench.report/v0`) and Markdown: hosts only, nearest-rank
  p50/p95/p99, error counts by kind, a per-second timeline, the definitions, and the
  no-verdict rule — more than 10% of viewers failing to join invalidates the aggregate.
  A hand-written Prometheus `/metrics` during a run (WB-12).
- `internal/testserver`, an in-process WHIP/WHEP relay on pion, so `go test -race ./...`
  runs real publisher → relay → viewer round trips on loopback, including a relay that
  strips the extension, one that refuses viewers and one that drops packets (WB-13).
- CI, CodeQL, release by tag with checksums and build-provenance attestations, release
  drift, Pages from the README, backlog-to-issues sync, a leak check over the tracked
  files and the whole history, and a dated smoke run against MediaMTX v1.21.1 in
  `evals/` (WB-14).
