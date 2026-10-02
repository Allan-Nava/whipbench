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
- `- [ ]` open, `- [x]` shipped with `ver=x.y.z` (or `ver=main` when merged, unreleased);
  decided against → ticked with `ver=dropped` and the reason in the body.
- Labels: `client`, `measurement`, `benchmark`, `report`, `research`, `release`, `docs`,
  `project`, `tests`, `enhancement`.

## v0.1.0 — The latency method, and a first report that means something <!-- ms: phase=now -->

0.0.1 measures one-way delay only where the server forwards abs-capture-time, and the
first server tried does not negotiate it. This milestone decides how whipbench measures
latency — through the network, and glass to glass — runs it against MediaMTX, and
publishes that report. **No v0.1.0 before WB-1 has a decided method and WB-5 is in
`evals/`.**

- [ ] **WB-1 — Latency method: run QRSPI on it**: one fresh session per phase, starting
  from the empty Questions artifact in `thoughts/WB-1-latency-method/`. What it must
  settle: whether abs-capture-time stays the network stamp or a custom extension is
  needed for servers that negotiate only what they know; whether glass-to-glass is
  measured through a timestamp drawn into the frames (WB-2); what a viewer reports when
  the two disagree; and what "latency" means in a report so two servers can be
  compared. <!-- wb: prio=high size=L labels=research,measurement -->
- [ ] **WB-2 — Visual timestamp for glass-to-glass**: the open design question. Encode
  the send time into the picture (a binary strip, a QR-like block, a frame counter) so a
  viewer that decodes can read it back after the server's own pipeline — which is the
  only latency a server that strips extensions or transcodes cannot hide. Needs a
  decoder in the viewer (cgo, or a pure-Go VP8 decoder) and so collides with the
  pure-Go rule; the cost per viewer decides whether it runs on every viewer or a sample.
  <!-- wb: prio=high size=L labels=research,measurement -->
- [ ] **WB-3 — Clock synchronisation between machines**: one-way delay across two hosts
  is only as good as their clock agreement. Decide what whipbench requires (NTP, chrony,
  PTP), how it measures the offset it is running with (an NTP query at start and end, or
  an RTT-halving exchange with the other side), and how the report carries that error
  bar instead of a bare number. <!-- wb: prio=high size=M labels=research,measurement -->
- [ ] **WB-4 — Header extensions, server by server**: for MediaMTX, OvenMediaEngine,
  LiveKit and Janus, record whether the WHIP and WHEP answers negotiate abs-capture-time,
  whether the server forwards it, rewrites it or strips it, and which extensions it does
  forward. MediaMTX v1.21.1 negotiates it on neither leg (2026-10-01). The table decides
  how much of WB-1 can rest on an extension at all. <!-- wb: prio=high size=M labels=research,benchmark -->
- [ ] **WB-5 — Live run against MediaMTX with the decided method**: the 0.0.1 smoke run
  repeated with WB-1's method, published in `evals/` as the first report that carries a
  latency figure or says, with evidence, why it cannot. <!-- wb: prio=high size=M labels=benchmark -->
- [ ] **WB-6 — Ethics of load-testing managed services**: write down the rule the README
  states in one line — only servers you run, a managed service only on your own account
  and within its terms, or with written permission — and what a run against a managed
  service must record (plan, region, the permission). No managed-service numbers are
  published before this exists. <!-- wb: prio=high size=S labels=docs,research -->
- [x] **WB-7 — Move the backlog tooling to backlogsync**: `scripts/backlog.mjs` and
  `.github/workflows/backlog-issues.yml` are copied from a sibling repository for now.
  Replace both with `Allan-Nava/backlogsync` once that tool reaches 0.1.0, keeping the
  `WB-n` ids and the issue titles unchanged. Done 2026-10-01: the CI `backlog`
  job, `backlog-issues.yml` and `release-drift.yml` on backlogsync `@backlogsync--v0.1.0`,
  `npm run backlog` / `npm run roadmap` on `npx backlogsync@0.1.0`; the old script, its test
  and fixtures removed. <!-- wb: prio=low size=S labels=project ver=main -->
- [ ] **WB-8 — Ramp phase against the GOP**: the ramp is deterministic, so when its step
  is a multiple of the 1 s GOP every viewer arrives at the same point of it and join
  times cluster (the 10-viewer MediaMTX run: p50 1003 ms, min 1001 ms). Add an optional
  seeded offset per viewer, recorded in the report, so join time samples the GOP evenly
  whatever the ramp. <!-- wb: prio=med size=S labels=measurement -->
- [ ] **WB-38 — One-way delay by frame fingerprint**: the headline delay figure, and the
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
  ceiling). <!-- wb: prio=high size=L labels=measurement,client -->

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
  extensions, so that a retransmitted first packet still carries the stamp.
  <!-- wb: prio=med size=M labels=measurement,client -->
- [ ] **WB-40 — Topology, clock and comparability in the report**: what makes two delay
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
  there is no merged best source. <!-- wb: prio=high size=M labels=report,measurement -->
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

## v0.3.0 — The comparative report across four servers <!-- ms: phase=later -->

- [ ] **WB-20 — Four servers, one scenario set**: MediaMTX, OvenMediaEngine, LiveKit and
  Janus, each in Docker on the same machine (and then on separate machines with WB-3's
  clock discipline), the same scenarios, every run in `evals/`.
  <!-- wb: prio=high size=L labels=benchmark -->
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
  merges their reports into one, per host and in total. Needs WB-3's clock discipline;
  an agent whose clock offset is unknown contributes no latency samples.
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
  server accepts it. <!-- wb: prio=high size=M labels=client ver=main -->
- [x] **WB-10 — WHEP viewers**: N concurrent native viewers; join time to the first RTP
  packet and the first complete keyframe, loss from sequence numbers, RFC 3550 jitter,
  keyframe interval, stalls, one-way delay or the reason it is unavailable.
  <!-- wb: prio=high size=M labels=client,measurement ver=main -->
- [x] **WB-11 — Scenarios and the ramp**: `whipbench run scenario.json` with `viewers`,
  `rampSeconds`, `holdSeconds`, strict JSON, `whipbench view` for viewers alone.
  <!-- wb: prio=high size=S labels=client ver=main -->
- [x] **WB-12 — Reports**: JSON and Markdown, hosts only, nearest-rank p50/p95/p99, error
  counts, a per-second timeline, the definitions carried in the report, the no-verdict
  rule, and a hand-written Prometheus `/metrics`. <!-- wb: prio=high size=M labels=report ver=main -->
- [x] **WB-13 — Tests without a server**: an in-process WHIP/WHEP relay on pion
  (`internal/testserver`) for real publisher → relay → viewers round trips on loopback,
  plus the loss, jitter, ramp, no-verdict and redaction maths on synthetic input.
  <!-- wb: prio=high size=M labels=tests ver=main -->
- [x] **WB-14 — Repo operating model and a smoke run**: CI (vet, race tests, lint,
  cross-builds, the leak check over the history), release by tag with checksums and
  attestations, release drift, CodeQL, Pages from the README, backlog sync; MediaMTX
  measured locally in `evals/`. <!-- wb: prio=med size=M labels=project,release ver=main -->
