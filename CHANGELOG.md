# Changelog

All notable changes to whipbench. The format is [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versions follow [SemVer](https://semver.org/). Items reference their `WB-n` backlog id.

## [Unreleased]

### Added
- A seeded per-viewer offset on the ramp (WB-8): `rampOffsetSeed` and
  `rampOffsetMaxSeconds` in the scenario, `--ramp-offset-seed` and `--ramp-offset-max` on
  `view` and `run`. Each viewer's start moves by an offset drawn uniformly from
  [0, bound) — 1 s by default, the clip's GOP — by SplitMix64 seeded with the key, so join
  time samples the GOP evenly whatever the ramp step. The report records the seed, the
  bound and every viewer's `rampOffsetMs`, plus a Method line; the same seed gives the
  same offsets. Off by default: without the key the ramp and the report are unchanged.

### Changed
- The 0.0.1 delay figure is renamed **packet transit** (WB-1) — per packet, arrival
  minus the abs-capture-time send stamp — so no metric is called "latency" and "one-way
  delay" is left for the per-frame figure of WB-38. Breaking: the report key `latency`
  is now `packetTransit` (the schema stays `whipbench.report/v0`, since no key changed
  meaning), the Prometheus histogram `whipbench_one_way_delay_seconds` is now
  `whipbench_packet_transit_seconds`, and the Markdown and stdout say "packet transit".
  A 0.0.1 parser or dashboard meets a missing key, not an error; there is no alias.
- The backlog check, the roadmap, the issue sync and the release-drift check are
  [backlogsync](https://github.com/Allan-Nava/backlogsync) 0.1.0, configured in
  `package.json#backlogsync`; `scripts/backlog.mjs`, its test and fixtures are gone, and
  `npm run roadmap` regenerates `ROADMAP.md` (WB-7).

### Security
- golang.org/x/crypto 0.48.0 → 0.52.0 and x/net → 0.55.0 (x/sys 0.45.0), after
  Dependabot security alerts on the first push.

### Fixed
- CI lint builds golangci-lint with the module's Go 1.27 instead of using the release
  binary, which is built with Go 1.26 and refuses the module; staticcheck and unused
  are disabled there until their IR builder handles Go 1.27's standard library.
- The leak check allows support@github.com, which Dependabot's commits carry.

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
