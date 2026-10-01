# Backlog — whipbench

Single source of truth for what is planned. Items keep a stable `WB-n` id so commits,
the CHANGELOG, the `thoughts/` artifacts and the issues can reference them.

[ROADMAP.md](ROADMAP.md) is a **generated** view of this file — run
`node scripts/backlog.mjs roadmap` after touching it, or CI fails. The GitHub issues are
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
- [ ] **WB-7 — Move the backlog tooling to backlogsync**: `scripts/backlog.mjs` and
  `.github/workflows/backlog-issues.yml` are copied from a sibling repository for now.
  Replace both with `Allan-Nava/backlogsync` once that tool reaches 0.1.0, keeping the
  `WB-n` ids and the issue titles unchanged. <!-- wb: prio=low size=S labels=project -->
- [ ] **WB-8 — Ramp phase against the GOP**: the ramp is deterministic, so when its step
  is a multiple of the 1 s GOP every viewer arrives at the same point of it and join
  times cluster (the 10-viewer MediaMTX run: p50 1003 ms, min 1001 ms). Add an optional
  seeded offset per viewer, recorded in the report, so join time samples the GOP evenly
  whatever the ramp. <!-- wb: prio=med size=S labels=measurement -->

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
