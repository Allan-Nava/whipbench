# CLAUDE.md

Guidance for Claude Code (and any coding agent — [AGENTS.md](AGENTS.md) points here)
when working in this repository.

## What this repo is

`whipbench` is a vendor-neutral WebRTC benchmark over WHIP (RFC 9725, ingest) and WHEP
(playback). A native Go publisher streams a pre-encoded synthetic clip; native Go
viewers on pion/webrtc v4 play it back; a report compares servers with fixed
definitions. One static binary, `CGO_ENABLED=0`, no browser.

## Layout

```
clips.go                 package whipbench: the two clips, embedded with go:embed
cmd/whipbench/           the CLI: publish, view, run, version; exit 0/1/2/3
internal/clip/           IVF and Annex-B parsing into loopable frames, RTP timestamps
internal/rtc/            the one pion API every peer uses: codecs, abs-capture-time, ICE options
internal/whip/           the HTTP half of WHIP/WHEP; errors that never carry a URL
internal/publisher/      WHIP publisher: paced loop, send-time stamp per packet
internal/viewer/         WHEP viewer: join times, stats, packet transit or why not
internal/rtpstats/       loss, jitter, keyframes, stalls — pure arithmetic (RFC 3550)
internal/stats/          nearest-rank summaries and the mergeable delay histogram
internal/scenario/       the scenario file, defaults, validation, the ramp and its seeded offsets (WB-8)
internal/runner/         one run: publisher, warmup, ramp, hold, timeline, metrics server
internal/report/         JSON + Markdown, hosts only, the no-verdict rule
internal/metrics/        live counters and the hand-written Prometheus exposition
internal/testserver/     in-process WHIP/WHEP relay for the end-to-end tests
internal/version/        the version string reports carry
testdata/                clip-vp8.ivf, clip-h264.h264 (made by scripts/make-clips.sh)
examples/                scenario files for a local MediaMTX
evals/                   dated live runs: a Markdown summary plus the raw reports
docs/                    load-testing-etiquette.md: the rule live runs follow (WB-6)
thoughts/                QRSPI artifacts (WB-1 starts with an empty Questions file)
scripts/                 make-clips.sh, check-repo.sh, leakcheck.mjs (+ test)
site/build.mjs           the Pages site, generated from README.md
```

## The rules the code encodes

Do not weaken these; they are what makes a number from whipbench worth quoting.

1. **A measurement has a definition before it has code.** README table, the `Method`
   lines in `internal/report/report.go`, and the package comment say the same thing.
2. **Never a fake number.** When the input a measurement needs is missing — the header
   extension not negotiated or stripped, implausible stamps, too many viewers failed —
   the report says *unavailable* or *no verdict* and why. No fallback estimate.
3. **The no-verdict rule:** more than 10% of the viewers failing to join invalidates
   the aggregate (`report.NoVerdictThreshold`). So does a publisher that never streamed
   and an interrupted run. `check-repo.sh` holds the README to the constant.
4. **Hosts only.** No path, query string, user info, `Location` or token reaches a
   report, an error message or the console. `whip.Host` and `whip.Scrub` are the only
   ways out; the tests assert it on real round trips.
5. **Packet transit and one-way delay are network plus server, not glass-to-glass**,
   and they need one clock or synchronised clocks. No metric is called "latency" (WB-1);
   one-way delay is WB-38 and capture-to-decode WB-2, neither implemented yet.
6. **Pure Go, no cgo.** A decoder in the viewer (WB-2) has to respect it or argue
   against it in the Design phase.

## Facts the code depends on (dated)

- **abs-capture-time** (`http://www.webrtc.org/experiments/rtp-hdrext/abs-capture-time`):
  64-bit UQ32.32 NTP timestamp, optionally followed by a 64-bit clock offset; pion/rtp
  v1.10.5 implements it as `rtp.AbsCaptureTimeExtension`. pion/webrtc v4.2.22 has no URI
  constant for it, hence `rtc.AbsCaptureTimeURI` (2026-10-01).
- **pion's ivfreader** converts frame pts with `pts·den/num`, which is not seconds;
  `clip.ParseIVF` reads IVF itself so the frame duration is `step·scale/rate` (2026-10-01).
- **pion's TrackLocalStaticRTP** rewrites SSRC and payload type per binding and leaves
  sequence numbers, timestamps and extensions alone; its interceptors append their own
  extensions to the header they are given, so a relay builds a fresh header per leg
  (`internal/testserver`) (2026-10-01).
- **MediaMTX v1.21.1** negotiates abs-capture-time on neither its WHIP nor its WHEP
  answer; in Docker on macOS it needs `MTX_WEBRTCADDITIONALHOSTS=127.0.0.1` and the UDP
  port published for a client on the host to connect (2026-10-01, `evals/`).
- **golangci-lint v2.12.2 on Go 1.27**: the release binary is built with Go 1.26 and
  refuses the module; built with `goinstall`, its staticcheck/unused IR builder
  (honnef.co/go/tools v0.7.0) panics on Go 1.27's `internal/poll`, so CI runs it with
  those two disabled (2026-10-01). Lift that when a release handles 1.27.
- **RFC 3550**: extended sequence numbers and loss in A.1/A.3, interarrival jitter in
  §6.4.1/A.8. **RFC 9725**: POST an SDP offer, 201 with the answer and `Location`,
  DELETE the resource to end the session.
- **The two tools whipbench is positioned against** (read 2026-10-01): Softvelum
  whep-load-tester — Go, pion, WHEP playback capacity, no WHIP, no documented latency
  measurement; webrtcperf — Puppeteer-driven Chromium, latency from a frame watermark
  and abs-capture-time, AGPL.

## Verifying a change

```bash
gofmt -l . && go vet ./... && go test -race -count=1 ./...; echo "exit $?"
golangci-lint run ./...; echo "exit $?"
./scripts/check-repo.sh; echo "exit $?"
node scripts/leakcheck_test.mjs && node scripts/leakcheck.mjs; echo "exit $?"
npm run backlog && npm run build:site
```

Print the exit code of every test and check, and read the whole output — never judge a
run through `| tail`, which keeps the pipe's status, not the command's.

Against a real server, locally only: see the README's quick start, and record a run
worth keeping in `evals/YYYY-MM-DD-<what>.md` with the server version and host. A
managed service only on an own account within its terms, or with written permission,
and its evals file adds the plan, the region and that permission
([docs/load-testing-etiquette.md](docs/load-testing-etiquette.md), WB-6).

## Publishing hygiene

Before every push: `node scripts/leakcheck.mjs` with the private denylist in
`LEAKCHECK_DENYLIST` (it is not in this repository, by design), which covers `git grep`
over the tracked files and `git log -p` over the whole history. Nothing private — home
paths, file URLs, e-mail addresses other than the author's public one, private IPs,
internal hostnames, credentials, names of employers or customers — and no
tool-attribution trailers or footers in commits, pull requests or files.

## Conventions

- BACKLOG.md first: every idea is a `WB-n` item; shipped items say `ver=`. Regenerate
  ROADMAP.md with `npm run roadmap`; `npm run backlog` (backlogsync) fails when it is stale.
- CHANGELOG under `[Unreleased]` in the same pull request as the change.
- Prose in English, British-leaning spelling, em-dashes, no marketing filler, no
  decorative emoji.
- A dependency needs a reason written here. Today: pion/webrtc v4 (the WebRTC stack —
  the point of the project). The Prometheus exposition is hand-written to avoid the
  client library.
