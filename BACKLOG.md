# Backlog — whipbench

Single source of truth for what is planned. Items keep a stable `WB-n` id so commits,
the CHANGELOG, the `thoughts/` artifacts and the issues can reference them.

[ROADMAP.md](ROADMAP.md) is a **generated** view of this file — run
`npm run roadmap` (backlogsync) after touching it, or CI fails. The GitHub issues are
synced from it one way on every push to `main` that changes this file.

## How to write an item

```
## v0.2.0 — Title of the milestone <!-- ms: phase=next -->

- [ ] **WB-99 — Short name**: what it is, why it earns its place, what it needs to
  touch. <!-- wb: prio=high size=M labels=client -->
```

- The **id never changes**; a new item takes the next free number.
- `- [ ]` open, `- [x]` shipped with `ver=x.y.z` (or `ver=0.1.0` when merged, unreleased);
  decided against → ticked with `ver=dropped` and the reason in the body.
- Labels: `client`, `measurement`, `benchmark`, `report`, `research`, `release`, `docs`,
  `project`, `tests`, `enhancement`.

## v0.1.0 — The latency method, and a first report that means something <!-- ms: phase=now -->

0.0.1 measures packet transit only where the server forwards abs-capture-time, and the
first server tried does not negotiate it. This milestone decides how whipbench measures
delay — WB-1 chose one-way delay per frame, by frame fingerprint (WB-38) — runs it
against MediaMTX, and publishes that report. **No v0.1.0 before WB-1 has a decided
method and WB-5 is in `evals/`.**

- [x] **WB-1 — Latency method: run QRSPI on it**: one fresh session per phase, starting
  from the empty Questions artifact in `thoughts/WB-1-latency-method/`. What it must
  settle: whether abs-capture-time stays the network stamp or a custom extension is
  needed for servers that negotiate only what they know; whether glass-to-glass is
  measured through a timestamp drawn into the frames (WB-2); what a viewer reports when
  the two disagree; and what "latency" means in a report so two servers can be
  compared. Done 2026-10-02:
  all six phases in `thoughts/WB-1-latency-method/` (#36–#48), each approved by the
  maintainer; the decision is one-way delay per frame by fingerprint (WB-38), the
  abs-capture-time stamp once per frame (WB-39), topology and clock in the report (WB-40),
  the window and retransmissions beside it (WB-41); the 0.0.1 metric is packet transit.
  <!-- wb: prio=high size=L labels=research,measurement ver=0.1.0 -->
- [x] **WB-3 — Clock exchange between publish and view**: how a split run, publisher and
  viewers on different hosts, measures the clock offset it runs with (WB-1, D6).
  `whipbench publish` answers a small clock responder, opt-in by flag, and
  `whipbench view --clock-peer HOST:PORT` runs an RTT-halving exchange against it at
  start, at end and every 30 s: offset ± min-RTT/2, piecewise-linear between the
  points, written into WB-40's `clock` block. Without `--clock-peer` the method is
  `none`, the delay `comparable: false`, "publisher clock not measured". The responder
  opens a second port between the load hosts; reports still carry no host names. RTCP
  sender reports and `chronyc` were rejected: SFUs originate their own SRs, and chrony
  reports each daemon's view of its upstream, not of the peer.
  Done 2026-10-05: `publish --clock-listen` and `view --clock-peer` over UDP
  (`internal/clocksync`), 16 probes per point, points before, every 30 s and after the
  viewers, method `exchange` with `points`, the first point's offset and the largest
  RTT/2 as uncertainty, a step making it not comparable; a silent peer gives `none` with
  "clock peer did not answer". No figure applies the offset yet — WB-39 is first.
  <!-- wb: prio=high size=M labels=client,measurement ver=0.1.0 -->
- [x] **WB-4 — What each server forwards: extensions, payload bytes, marker bit**: for
  MediaMTX, OvenMediaEngine, LiveKit and Janus, record whether the WHIP and WHEP answers
  negotiate abs-capture-time, whether the server forwards it, rewrites it or strips it,
  and which extensions it does forward (WB-39 rests on it); whether it forwards each
  depacketised VP8 frame, and each H.264 frame's VCL NAL units, byte for byte (WB-38's
  fingerprint rests on it); and whether it keeps the marker bit on each frame's last
  packet (WB-38's frame end). MediaMTX v1.21.1 negotiates abs-capture-time on neither
  leg (2026-10-01), and forwards frames byte for byte with one marker per frame on both
  codecs (2026-10-02, `evals/2026-10-02-mediamtx-fingerprint.md`); the other three are
  unverified. Done 2026-10-04 (`evals/2026-10-04-server-forwarding.md`): no server
  negotiates abs-capture-time on either leg; Janus 1.1.2 behind Meetecho's WHIP/WHEP
  servers forwards frames byte for byte with their markers, through a Streaming mountpoint
  since its VideoRoom WHEP needs a server offer; OvenMediaEngine v0.21.0 takes WHIP but has
  no WHEP (404), and LiveKit has neither without Ingress transcoding — whipbench cannot view
  either (WB-45). <!-- wb: prio=high size=M labels=research,benchmark ver=0.1.0 -->
- [x] **WB-5 — Live run against MediaMTX with the decided method**: the 0.0.1 smoke run
  repeated with WB-1's method, one-way delay by frame fingerprint (WB-38), published in
  `evals/` as the first report that carries a one-way delay figure or says, with
  evidence, why it cannot. Done 2026-10-05 (`evals/2026-10-05-mediamtx-one-way-delay.md`):
  the three smoke scenarios at `28dcd49`, every complete frame sampled, comparable, no step —
  p50 2.5 ms with 10 viewers, 4.4 ms (VP8) and 5.1 ms (H.264) with 50; client and server on
  one laptop, so the growth with viewers is not attributed (WB-25).
  <!-- wb: prio=high size=M labels=benchmark ver=0.1.0 -->
- [x] **WB-6 — Ethics of load-testing managed services**: write down the rule the README
  states in one line — only servers you run, a managed service only on your own account
  and within its terms, or with written permission — and what a run against a managed
  service must record (plan, region, the permission). No managed-service numbers are
  published before this exists. Done 2026-10-03: `docs/load-testing-etiquette.md` — the
  rule, what to check before a managed run, and the fields its evals file records
  (service, plan, region, permission, window); linked from the README's Load-testing
  etiquette section, the usage text, CONTRIBUTING and CLAUDE.md; `check-repo.sh` holds
  the README to the rule. <!-- wb: prio=high size=S labels=docs,research ver=0.1.0 -->
- [x] **WB-7 — Move the backlog tooling to backlogsync**: `scripts/backlog.mjs` and
  `.github/workflows/backlog-issues.yml` are copied from a sibling repository for now.
  Replace both with `Allan-Nava/backlogsync` once that tool reaches 0.1.0, keeping the
  `WB-n` ids and the issue titles unchanged. Done 2026-10-01: the CI `backlog`
  job, `backlog-issues.yml` and `release-drift.yml` on backlogsync `@backlogsync--v0.1.0`,
  `npm run backlog` / `npm run roadmap` on `npx backlogsync@0.1.0`; the old script, its test
  and fixtures removed. <!-- wb: prio=low size=S labels=project ver=0.1.0 -->
- [x] **WB-8 — Ramp phase against the GOP**: the ramp is deterministic, so when its step
  is a multiple of the 1 s GOP every viewer arrives at the same point of it and join
  times cluster (the 10-viewer MediaMTX run: p50 1003 ms, min 1001 ms). Add an optional
  seeded offset per viewer, recorded in the report, so join time samples the GOP evenly
  whatever the ramp. Done 2026-10-03: `rampOffsetSeed` and `rampOffsetMaxSeconds`
  (default 1 s) in the scenario, `--ramp-offset-seed` / `--ramp-offset-max` on `view` and
  `run`; offsets uniform in [0, bound) from SplitMix64, written out in
  `internal/scenario` and pinned by its reference vectors; the report records the seed,
  the bound and each viewer's `rampOffsetMs`. Off by default.
  <!-- wb: prio=med size=S labels=measurement ver=0.1.0 -->
- [x] **WB-38 — One-way delay by frame fingerprint**: the headline delay figure, and the
  one WB-5 publishes (WB-1, D2 and D4 in `thoughts/WB-1-latency-method/02-design.md`). In
  `run` the publisher logs t0 = `time.Now()` just before each frame's first `WriteRTP`,
  under a 64-bit hash of the frame's depacketised bytes: the whole VP8 frame, and for
  H.264 the VCL NAL units only, because a server may add or repeat SPS/PPS. Every viewer
  reassembles frames (pion `samplebuilder`), hashes them the same way and takes the
  latest send of that hash before t1, the arrival of the frame's marker packet. One
  sample per complete frame, NACK-recovered packets included; incomplete frames are
  counted, not sampled. Monotonic clock at both ends, report keys under `oneWayDelay`,
  source `fingerprint`; no header extension and no decoder, so it survives a server that
  drops abs-capture-time. The report states its limit: a frame later than the 4 s clip
  loop is ambiguous, and its sample is invalid. Byte-identical frames are excluded when
  the clip loads; a bitstream rewrite counts as `unmatchedFrames`; a server that drops
  the marker bit (WB-4) ends the frame at the last packet before the next RTP timestamp.
  Open: the CPU per viewer of reassembly and hashing at scale is unmeasured (WB-25's
  ceiling). Done 2026-10-03: QRSPI in `thoughts/WB-38-frame-fingerprint/` (#54–#76);
  two departures from this text, approved in the Design — whipbench's own reassembler
  instead of samplebuilder (D4), retransmissions beside the figure only with WB-41 (D6)
  — and the loop is counted in frames, not seconds (D3).
  <!-- wb: prio=high size=L labels=measurement,client ver=0.1.0 -->

## v0.2.0 — Simulcast, layer switches and metrics <!-- ms: phase=next -->

- [ ] **WB-15 — Simulcast publishing**: three encodings of the clip (rid h/m/l, pre-encoded
  at three sizes) on one WHIP session, so servers that do simulcast can be measured doing
  it. <!-- wb: prio=high size=L labels=client -->
- [ ] **WB-16 — Layer switches**: viewers that request a layer change (or have one forced
  by a bandwidth cap) and measure the time from the request to the first complete
  keyframe of the new layer, and the loss around the switch. <!-- wb: prio=high size=M labels=client,measurement -->
- [ ] **WB-17 — Metrics for long runs**: per-layer series, the client's own CPU and
  goroutine count (so a saturated client is visible), and an example Grafana dashboard
  over the existing `/metrics`. <!-- wb: prio=med size=M labels=report -->
- [ ] **WB-18 — Audio**: an Opus track in the clip and the viewers, with loss and jitter
  per track, and audio/video arrival skew. <!-- wb: prio=med size=M labels=client -->
- [ ] **WB-19 — STUN, TURN and trickle ICE**: ICE servers in the scenario, trickle ICE
  through WHIP/WHEP PATCH, so servers behind NAT and TURN-relayed viewers can be
  measured. <!-- wb: prio=low size=M labels=client -->
- [ ] **WB-39 — abs-capture-time once per frame**: the stamp source beside WB-38's
  fingerprint (WB-1, D3). The publisher writes abs-capture-time = t0 on each frame's
  first packet only, instead of on every packet, and the viewer joins it to its frame by
  RTP timestamp. In `run` the publisher keeps the values it sent, so a stamp outside that
  set is counted as rewritten, never sampled. Each leg is classified — publisher
  `negotiated` or `dropped`, viewer `forwarded`, `rewritten`, `dropped` or `unverified`
  (split runs) — and the figure is a wall-clock difference, exposed to clock steps
  (WB-40). It ships under `oneWayDelay` with source `stamp` and retires packet transit,
  the 0.0.1 per-packet figure, with its `packetTransit` key and its Prometheus series.
  Open: whether pion's NACK responder resends the stored packet with its header
  extensions, so that a retransmitted first packet still carries the stamp. It also owns
  WB-40's deferred `sourcesDisagree` flag and the block's `rewritten` count, which need
  the second source. WB-4 found no server that negotiates abs-capture-time on either leg
  (MediaMTX, OvenMediaEngine, Janus), so against those the source would only ever say
  `dropped`: worth building when a server in WB-45's set forwards it, not before.
  <!-- wb: prio=med size=M labels=measurement,client -->
- [x] **WB-40 — Topology, clock and comparability in the report**: what makes two delay
  figures comparable (WB-1, D6 and D8). Every report gains `topology` (`single-process`
  or `split`) and `clock` (`method`, `offsetMs`, `uncertaintyMs`, `stepDetected`), and no
  host names. In `run` the method is `monotonic` for the fingerprint or `same-wall-clock`
  for the stamp, and wall-clock and monotonic elapsed time more than 0.1 ms apart at run
  end set `stepDetected`; in `view` the offset comes from WB-3's exchange, or the method
  is `none` and the figure `comparable: false`. Each source gets one block — source,
  available, reason, frames, samples, invalid, rewritten, unmatched, the summary in ms,
  uncertaintyMs, comparable, notComparableReason — and never a number when unavailable.
  Two reports rank only if both are comparable, share scenario, clip and source, and
  carry an uncertainty of 1 ms or less. `sourcesDisagree` flags a forwarded stamp whose
  p50 differs from the fingerprint's beyond the uncertainty; nothing is averaged, and
  there is no merged best source.
  Done 2026-10-04: `topology` and `clock` on every report, `monotonic` with a step check in
  `run` — per one-second interval, beyond a 500 ppm slew, since the whole-run 0.1 ms bound
  flagged a laptop's steady 2.8 ppm drift on every run — and `none` in `view` (WB-3 fills it), `uncertaintyMs`, `comparable` and
  `notComparableReason` on every one-way delay block, and `report.Rankable` for WB-21;
  `rewritten` and `sourcesDisagree` wait for WB-39's second source.
  <!-- wb: prio=high size=M labels=report,measurement ver=0.1.0 -->
- [ ] **WB-41 — Sample window and retransmission beside delay**: what one-way delay is
  sampled over, and what sits next to it (WB-1, D7). A new scenario key
  `excludeFirstSeconds` (default 5) drops each viewer's first seconds, counted from its
  own first RTP packet; `warmupSeconds` keeps its meaning. Beside one-way delay go the
  NACKs each viewer sent (pion's stats interceptor, registered with the default
  interceptors in `internal/rtc/rtc.go` and never read) and `lateCompletedFrames`,
  frames whose gap was filled after their marker arrived; RTX stays unnegotiated. The
  1% histogram stays; the Method line says "±0.5 % of value" and values print to 0.1 ms.
  Open: whether the stats interceptor's NACK count means NACKs sent, and how recovered
  packets relate to `tooLate` in `internal/rtpstats/rtpstats.go`.
  <!-- wb: prio=med size=M labels=measurement,report -->
- [ ] **WB-42 — Live one-way delay series with a source label**: a Prometheus histogram of
  WB-38's per-frame figure, `whipbench_one_way_delay_seconds{source="fingerprint"}`, so a
  long run shows the headline while it runs. WB-38 opened none (its D8): an unlabelled
  series would change meaning when WB-39's `stamp` source lands, and
  `internal/metrics/metrics_test.go` keeps the name shut until the label exists.
  <!-- wb: prio=med size=S labels=report -->
- [x] **WB-43 — Frame rate for Annex-B clips from the clip**: `clip.Load` reads H.264 at a
  hard-coded 30 fps (`internal/clip/clip.go:231`), so a clip at another rate is paced and
  timestamped wrong; take the rate from the SPS timing information or a flag. One-way
  delay is safe either way — it counts the loop in frames (WB-38, D3) — the pacing is not.
  Done 2026-10-04: `internal/clip/sps.go` reads `timing_info` from the first SPS (profiles
  with scaling lists, every POC type, emulation prevention), `publish --fps` gives or
  overrides it, and a stream with neither is refused; checked on x264 clips at 24, 25,
  29.97 and 60 fps. <!-- wb: prio=low size=S labels=client ver=0.1.0 -->
- [ ] **WB-44 — One-way delay split by keyframe and delta frame**: a keyframe spans many
  packets and a delta frame few, so their first-to-last spread differs; report both
  distributions beside the pooled figure, as WB-38's Questions phase deferred (Q3).
  <!-- wb: prio=low size=M labels=measurement,report -->

## v0.3.0 — The comparative report across four servers <!-- ms: phase=later -->

- [ ] **WB-20 — Four servers, one scenario set**: MediaMTX, OvenMediaEngine, LiveKit and
  Janus, each in Docker on the same machine (and then on separate machines with WB-3's
  clock exchange), the same scenarios, every run in `evals/`. The set is WB-45's to
  decide: WB-4 found two of the four with no WHEP.
  <!-- wb: prio=high size=L labels=benchmark -->
- [ ] **WB-45 — Which servers the comparison can include**: WB-4 found OvenMediaEngine
  v0.21.0 with WHIP but no WHEP, and LiveKit with WHIP only through Ingress, which
  transcodes, and no WHEP — whipbench's viewer reaches neither. Decide WB-20's set among
  servers that speak both protocols with client offers (MediaMTX and Janus with Meetecho's
  servers are verified; candidates to check the same way include SRS and Broadcast Box),
  or give whipbench a viewer for one server's own signalling — a second code path whose
  figures must stay comparable with the WHEP viewer's. Record the choice and why in WB-20.
  <!-- wb: prio=high size=S labels=research,benchmark -->
- [ ] **WB-21 — `whipbench compare`**: reads several reports and renders them side by
  side, refusing to compare runs whose scenarios, clips or client machines differ, and
  printing a no-verdict run as no verdict. <!-- wb: prio=high size=M labels=report -->
- [ ] **WB-22 — The write-up**: what was measured, how, the numbers, what they do not
  show; published from the README to the site. <!-- wb: prio=med size=M labels=docs -->
- [ ] **WB-23 — A managed service on an own account**: one run against a managed WHIP/WHEP
  service on an account whipbench's author owns, within its terms, under WB-6's rule.
  <!-- wb: prio=low size=M labels=benchmark -->

## v0.4.0 — Load from many machines <!-- ms: phase=later -->

One client machine runs out before a server does. This milestone spreads a scenario over
several hosts and makes the client's own limit part of the report, so a saturated client
can never be mistaken for a slow server.

- [ ] **WB-24 — Coordinated multi-host runs**: `whipbench agent` on each load host and one
  coordinator that hands out the scenario, starts every agent at a shared instant, and
  merges their reports into one, per host and in total. Needs WB-3's clock exchange;
  an agent whose clock offset is unknown contributes no comparable one-way delay (WB-40).
  <!-- wb: prio=high size=L labels=client,report -->
- [ ] **WB-25 — The client's own ceiling**: calibrate how many viewers one machine sustains
  against the in-process relay before its own CPU, scheduler or socket buffers skew the
  numbers, record that ceiling in the report, and give no verdict on a run that exceeded
  it. <!-- wb: prio=high size=M labels=measurement,report -->
- [ ] **WB-26 — Container image and job specs**: a minimal image on ghcr.io, plus a
  Kubernetes Job and a Nomad batch job running agents, so a distributed run is one apply
  away. <!-- wb: prio=med size=M labels=release,docs -->

## v0.5.0 — whipbench in CI <!-- ms: phase=later -->

A server's latency regression should fail a pull request, not a launch. This milestone
turns a scenario into a test a server project can run on every change.

- [ ] **WB-27 — Assertions and their exit code**: thresholds in the scenario
  (`"assert": {"join.p95": "<1500ms", "loss": "<0.5%"}`) checked after the run, each
  reported pass or fail with its measured value, and a distinct exit code when one fails
  — a no-verdict run never passes an assertion. <!-- wb: prio=high size=M labels=report,enhancement -->
- [ ] **WB-28 — A GitHub Action**: `uses: Allan-Nava/whipbench@<tag>` that runs a scenario
  against a server started as a service container, uploads the report as an artifact and
  writes the summary to the job page. <!-- wb: prio=high size=M labels=release,enhancement -->
- [ ] **WB-29 — Against a baseline**: compare a run with a stored baseline report under
  `compare`'s refusal rules (WB-21) and its noise bounds, so a pull request shows the
  delta and only a difference larger than run-to-run noise fails.
  <!-- wb: prio=med size=M labels=report,measurement -->

## v0.6.0 — What the viewer actually sees <!-- ms: phase=later -->

Packets arriving is not video playing. Once viewers decode (WB-2), they can measure what
a person would notice — and the network can be made worse on purpose to see how each
server copes.

- [ ] **WB-2 — Keyframe capture-to-decode for VP8**: the third delay figure (WB-1, D5),
  built here beside transcoding (WB-31), where it first measures something WB-38 cannot.
  `scripts/make-clips.sh` draws a block code — a 16-bit frame index plus check bits,
  sized for 600 kbit/s — into each source frame, and the clips are regenerated once. Up
  to 10 sampled viewers decode keyframes only, with `golang.org/x/image/vp8` (BSD-3, pure
  Go, so the pure-Go rule holds), read the index back and take the send time from
  WB-38's send log, keyed by index: capture-to-decode is the keyframe's decode at the
  endpoint decoder's output minus t0. Unreadable codes are counted, never guessed;
  keyframes are the largest frames, so the figure is biased high and labelled so; no
  H.264. `methodsDisagree` flags it against WB-38's figure, never averaged. A vetted
  pure-Go inter-frame decoder would upgrade it to every frame. Open: whether a code
  drawn by `make-clips.sh` survives libvpx at 600 kbit/s and reads back after decode.
  <!-- wb: prio=med size=L labels=measurement,research -->
- [ ] **WB-30 — Freezes and frame drops**: from the decoded frames, the count and length of
  visible freezes and the frames that never displayed, per viewer and in total.
  <!-- wb: prio=high size=M labels=measurement -->
- [ ] **WB-31 — Picture quality through a transcoding server**: compare received frames with
  the source clip (PSNR, and VMAF where the binary is available) for servers that
  re-encode, on a sample of viewers, recorded with the method.
  <!-- wb: prio=med size=L labels=measurement,research -->
- [ ] **WB-32 — Impairment profiles**: named network conditions in the scenario — loss,
  added delay, jitter, a bandwidth cap — applied per viewer in the client itself, so the
  same profile behaves the same on every machine, and the report says which one ran.
  <!-- wb: prio=high size=M labels=client,measurement -->

## v1.0.0 — A report format others can depend on <!-- ms: phase=later -->

1.0 is a promise, not a feature count: the report, the scenario file and the CLI stop
changing under the people who build on them, and every published number can be
reproduced from the repository.

- [ ] **WB-33 — Report schema v1**: a published, versioned JSON Schema for the report, a
  compatibility rule (additive within v1), and `compare` reading every v1 report.
  <!-- wb: prio=high size=M labels=report,docs -->
- [ ] **WB-34 — Stable CLI and scenario keys**: flags and scenario keys frozen, a
  deprecation policy in CONTRIBUTING (warn for one minor, remove in the next major).
  <!-- wb: prio=high size=S labels=docs,project -->
- [ ] **WB-35 — Reproduce any published number**: `scripts/reproduce.sh <eval>` re-runs a
  report from `evals/` with the server image pinned by digest and the same scenario,
  clip and whipbench version, and prints the new report beside the old.
  <!-- wb: prio=high size=M labels=benchmark,tests -->
- [ ] **WB-36 — A results page**: the site renders `evals/` into one page — the method,
  each server's numbers with its version and date, and what the numbers do not show —
  generated, never edited by hand. <!-- wb: prio=med size=M labels=docs,report -->
- [ ] **WB-37 — Supply chain**: signed release artifacts with verified attestations, an
  SBOM per release, and `govulncheck` in CI, so a team that runs whipbench inside its
  own CI can check what it is running. <!-- wb: prio=med size=M labels=release,tests -->

## v0.0.1 — The first binary <!-- ms: phase=shipped -->

Not released: it exists so the clients, the report and the operating model can be
reviewed before the latency method is decided.

- [x] **WB-9 — WHIP publisher**: the embedded synthetic clip (VP8 IVF and H.264 Annex-B,
  regenerated by `scripts/make-clips.sh`), looped with continuous RTP timestamps, paced at
  the frame rate, every packet stamped with its send time in abs-capture-time when the
  server accepts it. <!-- wb: prio=high size=M labels=client ver=0.1.0 -->
- [x] **WB-10 — WHEP viewers**: N concurrent native viewers; join time to the first RTP
  packet and the first complete keyframe, loss from sequence numbers, RFC 3550 jitter,
  keyframe interval, stalls, one-way delay or the reason it is unavailable.
  <!-- wb: prio=high size=M labels=client,measurement ver=0.1.0 -->
- [x] **WB-11 — Scenarios and the ramp**: `whipbench run scenario.json` with `viewers`,
  `rampSeconds`, `holdSeconds`, strict JSON, `whipbench view` for viewers alone.
  <!-- wb: prio=high size=S labels=client ver=0.1.0 -->
- [x] **WB-12 — Reports**: JSON and Markdown, hosts only, nearest-rank p50/p95/p99, error
  counts, a per-second timeline, the definitions carried in the report, the no-verdict
  rule, and a hand-written Prometheus `/metrics`. <!-- wb: prio=high size=M labels=report ver=0.1.0 -->
- [x] **WB-13 — Tests without a server**: an in-process WHIP/WHEP relay on pion
  (`internal/testserver`) for real publisher → relay → viewers round trips on loopback,
  plus the loss, jitter, ramp, no-verdict and redaction maths on synthetic input.
  <!-- wb: prio=high size=M labels=tests ver=0.1.0 -->
- [x] **WB-14 — Repo operating model and a smoke run**: CI (vet, race tests, lint,
  cross-builds, the leak check over the history), release by tag with checksums and
  attestations, release drift, CodeQL, Pages from the README, backlog sync; MediaMTX
  measured locally in `evals/`. <!-- wb: prio=med size=M labels=project,release ver=0.1.0 -->
