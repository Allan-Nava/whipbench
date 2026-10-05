# Changelog

All notable changes to whipbench. The format is [Keep a Changelog](https://keepachangelog.com/en/1.1.0/);
versions follow [SemVer](https://semver.org/). Items reference their `WB-n` backlog id.

## [Unreleased]

### Added
- One-way delay split by keyframe and delta frame (WB-44): every one-way delay block, per
  viewer and pooled, gains `keyframes` and `deltaFrames`, each `samples` and `ms`, the
  samples split by whether the clip frame they matched is a keyframe. The split is made
  where the viewer records a sample, so `keyframes.samples + deltaFrames.samples =
  samples`; a half with no sample has no `ms`. The pooled figure stays the headline and
  keeps its comparability; the split carries none of its own, a new Method line says it is
  descriptive and never ranked, and the Markdown adds "one-way delay, keyframes" and
  "one-way delay, delta frames" under the pooled row. `fingerprint.Table.Key` says which
  clip frames are keyframes. Schema still `whipbench.report/v0`.

## [0.1.0] — 2026-10-05

The latency method, and a first report that means something. `run` reports one-way delay
per frame by frame fingerprint (WB-38), which needs no header extension and so works
against servers that forward none; every report says how its clock was set and whether the
figure may be ranked against another's (WB-40); split runs measure the publisher's clock
(WB-3). The first live figure against MediaMTX is in `evals/` (WB-5), with what four servers
forward (WB-4). The first tagged release.

### Added
- Clock exchange between `publish` and `view` (WB-3): `publish --clock-listen ADDR`
  answers a UDP exchange while it publishes — a 32-byte request, an answer of the same
  size carrying the responder's clock at receipt and at send, anything else dropped — and
  `view --clock-peer HOST:PORT` measures a point before the first viewer, every 30 s
  while viewers run and after they stop: the best of 16 probes 10 ms apart, the round
  trip on the monotonic clock, the offset ± RTT/2. The report's `clock` gains method
  `exchange`, `points` (`tS`, `offsetMs`, `rttMs`) and `reason`; `offsetMs` is the first
  point's, `uncertaintyMs` the largest RTT/2, and a step makes the figure not comparable,
  as for any wall-clock method. A peer that never answers leaves method `none` with
  `reason` "clock peer did not answer" and no number. The address reaches no report,
  metric or log line. New package `internal/clocksync`, with the piecewise-linear
  `Model` WB-39's stamp source will be the first to apply; no figure uses the offset yet.
  Beyond the item: the responder also drops a 32-byte packet whose padding is not zero,
  so two responders cannot be set answering each other, and a peer address that does
  not resolve gives `reason` "clock peer address did not resolve".
- `evals/2026-10-05-mediamtx-one-way-delay.md` (WB-5): the 0.0.1 smoke scenarios against
  MediaMTX v1.21.1 with WB-38's one-way delay — p50 2.5 ms (10 viewers), 4.4 ms (50, VP8)
  and 5.1 ms (50, H.264), every complete frame sampled, comparable, no step; the reports
  beside it.
- `evals/2026-10-04-server-forwarding.md` (WB-4): what MediaMTX, OvenMediaEngine, LiveKit
  and Janus forward. No server negotiates abs-capture-time on either leg; Janus 1.1.2 behind
  Meetecho's WHIP/WHEP servers forwards frames byte for byte with their markers, with the
  image recipe and both reports beside it; OvenMediaEngine v0.21.0 has no WHEP and LiveKit
  none without a transcoding ingress, so WB-45 now decides WB-20's server set.
- Topology, clock and comparability in the report (WB-40): every report gains `topology`
  (`single-process` for a `run` that publishes, `split` otherwise) and `clock` (`method`,
  `offsetMs`, `uncertaintyMs`, `stepDetected`), and no host names. `run` is `monotonic`,
  offset and uncertainty 0, with `stepDetected` set when, between two once-a-second
  observations, wall-clock and monotonic elapsed time differ by more than 0.1 ms plus a
  500 ppm slew. The design's bound on the whole run was dropped before release: a laptop's
  steady 2.8 ppm drift crossed it after 36 s and flagged every run (measured in WB-5's
  runs). `view` is `none` and records no offset or uncertainty
  until WB-3 measures the publisher's clock (`report.Input.Clock` is the seam). Each
  one-way delay block, per viewer and pooled, adds `uncertaintyMs` (only when available),
  `comparable` and `notComparableReason`. `report.Rankable` is the ranking rule WB-21 will
  call: both blocks comparable, both aggregates valid, the same clip (codec, loop frames)
  and every scenario key equal but the endpoint hosts, the name, the bearer variable and
  the metrics address, uncertainty 1 ms or less. The
  Markdown shows topology and clock in its header and marks a figure that is not
  comparable, with the reason; a Method line states the definition. The schema stays
  `whipbench.report/v0`: keys were added, none changed meaning. Departures from the
  backlog item: the block keeps WB-38's key names (`completeFrames`, `unmatchedFrames`)
  rather than the design's shorter ones, since renaming would change their meaning for
  a reader of earlier reports; `rewritten` belongs to the stamp source and arrives with
  WB-39; `sourcesDisagree` needs two sources and is deferred to WB-39; `same-wall-clock`
  is defined but unused until WB-39's stamp; a `run` without a WHIP endpoint publishes
  nothing and is `split`, not `single-process`; and a ranking also refuses a report
  whose aggregate has no verdict.
- One-way delay per frame, by frame fingerprint, in `run` (WB-38): first-packet send to
  last-packet arrival, one sample per complete frame, on the one process's monotonic
  clock. A new report key `oneWayDelay` — a list of source blocks, source `fingerprint`,
  per viewer and in the aggregate; the schema stays `whipbench.report/v0`, since a key was
  added and none changed meaning. The Markdown table and the stdout line lead with it,
  packet transit unchanged below; `view` has no send log and reports it unavailable. Two
  departures from the backlog item change what a reader gets: whipbench reassembles
  frames itself, with a 1 s window, instead of pion's samplebuilder (D4), and no
  retransmission count sits beside the figure until WB-41 (D6). No Prometheus series yet
  (WB-42).
- A seeded per-viewer offset on the ramp (WB-8): `rampOffsetSeed` and
  `rampOffsetMaxSeconds` in the scenario, `--ramp-offset-seed` and `--ramp-offset-max` on
  `view` and `run`. Each viewer's start moves by an offset drawn uniformly from
  [0, bound) — 1 s by default, the clip's GOP — by SplitMix64 seeded with the key, so join
  time samples the GOP evenly whatever the ramp step. The report records the seed, the
  bound and every viewer's `rampOffsetMs`, plus a Method line; the same seed gives the
  same offsets. Off by default: without the key the ramp and the report are unchanged.
- `docs/load-testing-etiquette.md` (WB-6): the rule the README states in one line —
  servers you run, a managed service only on your own account within its terms, or
  written permission — and what a run against a managed service records in `evals/`:
  service, plan, region, the permission and the run's window. The README section, the
  `whipbench` usage text, CONTRIBUTING and CLAUDE.md point at it, and `check-repo.sh`
  holds the README to the rule.

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
- An H.264 clip given to `publish --clip` was paced and timestamped at 30 fps whatever
  its rate. The rate is now the one its SPS declares (the VUI's `timing_info`: 25 fps is
  3600 ticks a frame, 29.97 is 3003), and the new `--fps` flag gives it when the SPS
  declares none, or overrides it; a stream with neither is refused rather than guessed.
  The embedded clips are unchanged, at 30 fps. One-way delay was right either way — it
  counts the loop in frames — but the pacing was not (WB-43).
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
