# whipbench — a vendor-neutral WebRTC benchmark over WHIP and WHEP

<p align="center"><img src="https://raw.githubusercontent.com/Allan-Nava/whipbench/main/assets/logo.svg" width="72" height="72" alt=""></p>

whipbench publishes a synthetic clip over **WHIP** (RFC 9725), plays it back with any number of **WHEP** viewers, and writes a report of what an operator cares about — one-way delay, join time, packet transit, packet loss, jitter, keyframe interval and run stability — with the same definitions whichever server sits in the middle. The clients are native Go on [pion/webrtc](https://github.com/pion/webrtc) v4, with no browser, so one machine can drive many viewers, and the report is meant to be laid next to another server's.

**Status: 0.1.0, the first release** ([binaries](https://github.com/Allan-Nava/whipbench/releases)). The clients, the report and the in-process test relay work, and the live runs against MediaMTX are in [`evals/`](evals/), the latest with a one-way delay figure (p50 2.5 ms with 10 viewers on one laptop, WB-5). MediaMTX does not negotiate the header extension 0.0.1's stamp travels in, so against it packet transit is reported **unavailable** — by design, never estimated. The delay method is decided ([WB-1](BACKLOG.md)) and built: `run` reports one-way delay per frame, by frame fingerprint, which needs no header extension ([WB-38](BACKLOG.md)), and every report says whether that figure may be ranked against another's ([WB-40](BACKLOG.md)); the first live figure is [WB-5](BACKLOG.md)'s.

## Install

From source, with Go 1.27 or later (pure Go, `CGO_ENABLED=0`):

```bash
go install github.com/Allan-Nava/whipbench/cmd/whipbench@latest
whipbench version
```

Or download a release binary — Linux and macOS, amd64 and arm64, static — from [the releases page](https://github.com/Allan-Nava/whipbench/releases), with a checksums file and build provenance you can verify:

```bash
gh attestation verify whipbench-v0.1.0-linux-amd64 --repo Allan-Nava/whipbench
```

The two synthetic clips are embedded in the binary; nothing else has to sit beside it.

## Quick start

A MediaMTX on this machine, in Docker. The extra host makes it advertise `127.0.0.1` as an ICE candidate, which a client outside the container can reach through the published UDP port:

```bash
docker run --rm -d --name mediamtx -e MTX_WEBRTCADDITIONALHOSTS=127.0.0.1 \
  -p 127.0.0.1:8889:8889 -p 127.0.0.1:8189:8189/udp bluenviron/mediamtx:latest

# One publisher, 50 viewers arriving over 10 s, held for 30 s, then a report.
whipbench run examples/mediamtx-local-50.json --out reports/
```

Or the two halves separately:

```bash
whipbench publish --whip http://127.0.0.1:8889/bench/whip --include-loopback
whipbench view    --whep http://127.0.0.1:8889/bench/whep -n 20 --ramp 5s --duration 30s --include-loopback
```

With the publisher and the viewers on different hosts, the two clocks differ, and `view` can measure by how much ([WB-3](BACKLOG.md)): `publish --clock-listen :7444` answers a small UDP clock exchange while it publishes, and `view --clock-peer HOST:PORT` runs that exchange against it before the first viewer starts, every 30 s while the viewers run, and after they stop. The report records the offset, its uncertainty and every point under `clock`, and never the address:

```bash
whipbench publish --whip http://192.0.2.10:8889/bench/whip --clock-listen :7444     # on the publishing host
whipbench view    --whep http://192.0.2.10:8889/bench/whep -n 20 --clock-peer 192.0.2.20:7444
```

`--clock-listen` opens a UDP port and answers anyone who can reach it with three timestamps — its own clock at receipt and at send, and the sender's echoed back — and nothing else: an answer is never larger than the request, and anything that is not a 32-byte request is dropped unanswered. It still tells whoever asks what this host's clock reads, and it is a second port between the load hosts, so bind it to the one interface the viewers' host reaches it on and let a firewall admit only that host.

`run` and `view` print the verdict and write `whipbench-<UTC time>-<name>.json` and `.md`. The exit status is 0 for a valid run, 3 for a run with no verdict, 1 for an error and 2 for a usage error, so a CI job can gate on it.

## What it measures

Every viewer records its own numbers; the report adds aggregates over the viewers that joined. Definitions, because a benchmark number without one cannot be compared:

| measure | definition |
|---|---|
| join: first RTP packet | WHEP POST sent → first RTP packet received |
| join: first keyframe | WHEP POST sent → last packet of the first keyframe received **complete** (every sequence number from its first packet to its marker). No decoder runs, so this is the earliest moment a frame *could* be decoded, not a decoded frame. A viewer has **joined** when it gets here within the join timeout (10 s by default). |
| signalling | WHEP POST sent → SDP answer received |
| loss | expected = highest − first extended sequence number + 1 (RFC 3550 A.1); lost = expected − received; duplicates are not counted as received. A packet retransmitted after a NACK counts as received, so lost is what never arrived — see "Loss and retransmission" below |
| NACKs sent | `nacksSent`: the RTCP NACK messages each viewer sent, summed in the aggregate — messages, not packets asked for ([WB-41](BACKLOG.md)) |
| jitter | RFC 3550 §6.4.1 interarrival jitter, J += (\|D\| − J)/16 on every packet, in milliseconds |
| keyframe interval | spacing of keyframe starts in RTP time — the GOP the server actually delivers |
| one-way delay | per frame, first-packet send to last-packet arrival: the publisher logs when it hands the frame's first packet to the stack, and the viewer reassembles the frame, hashes it — SHA-256 of the VP8 frame, or of the H.264 VCL NAL units (types 1-5, pending a check against H.264 Table 7-1) — and subtracts that send time from its last packet's arrival. A match the RTP timestamps prove whole loops too new, or one with no logged send, is invalid. One sample per complete frame after each viewer's first `excludeFirstSeconds` (5 s by default), on the one process's monotonic clock; `run` only — `view` reports it unavailable |
| packet transit | arrival time − the publisher's send-time stamp, per packet — not the one-way delay of a frame; see the next section |
| stability | stalls (a gap of 500 ms or more between packets), viewers dropped after joining, publisher schedule slips, and a per-second timeline of active viewers and packets |

Percentiles are nearest-rank: p95 is a value some viewer actually had. Join, loss and jitter take one value per joined viewer; one-way delay and packet transit both pool every sample of every viewer that has one — every sampled frame, every packet — in a histogram of 1 % buckets read at the geometric midpoint, so a percentile is within ±0.5 % of value (min, max and mean exact); the Markdown prints them to 0.1 ms.

Beside the pooled one-way delay figure, which stays the headline, every one-way delay block — per viewer and pooled — splits the same samples by the clip frame each one matched: `keyframes` and `deltaFrames`, each `samples` and `ms`, so the two counts add up to `samples` ([WB-44](BACKLOG.md)). A keyframe spans many more packets than a delta frame, and the spread from its first packet to its last is part of its sample, so the two distributions differ for reasons that are not forwarding. A half with no sample has no `ms` — never a zero — and the split carries no comparability of its own: it describes the figure, it is not a separate source, and it is never ranked. The Markdown prints it as two rows under the pooled one, "one-way delay, keyframes" and "one-way delay, delta frames".

**Sample window and retransmission ([WB-41](BACKLOG.md)).** A viewer's first seconds are not sampled: a frame whose first packet arrived within `excludeFirstSeconds` (5 s by default, 0 to sample from the first frame) of that viewer's own first RTP packet is still hashed and matched, but counts in `excludedFrames` instead of giving a sample, so `completeFrames` = `samples` + `invalid` + `unmatchedFrames` + `excludedFrames` + the frames that are clip duplicates. The window runs per viewer, so a ramp loses the same share of every viewer however late it arrives; `warmupSeconds`, the pause before the first viewer, keeps its meaning. It applies to the delay samples alone — the keyframe and delta-frame split and the live series see the same samples — while loss, jitter and join count from the first packet. Beside the delay sit `lateCompletedFrames`, complete frames one of whose packets arrived after the frame's last packet (its marker): a gap filled after the end had arrived, by a NACK retransmission or by reordering. Their sample still ends at that last packet; a frame whose last packet was itself the one retransmitted is not counted there, and its sample ends at the retransmission. And `nacksSent`, the NACK messages each viewer sent. pion's own stats interceptor cannot give that number: the default interceptors register the NACK generator before it, so it never sees the generator's NACKs, and whipbench counts them with a small interceptor registered ahead of the defaults (`rtc.NACKCounter`). RTX stays unnegotiated, so a retransmission is the original packet again and cannot be told from a reordered one.

The publisher streams a pre-encoded clip, 640×360 at 30 fps, 4 s, a keyframe every 30 frames, in VP8 or constrained-baseline H.264. [`scripts/make-clips.sh`](scripts/make-clips.sh) records the exact ffmpeg command and remakes them. Frames go on the wire byte for byte, paced at the frame rate, and the loop is seamless: frame *k* has RTP timestamp base + 3000·*k* whichever pass it belongs to. `publish --clip` streams a file of your own at the rate it declares: an IVF file's time base, or an H.264 Annex-B stream's SPS (the VUI's `timing_info`, so 29.97 fps is 3003 ticks a frame); `--fps` gives the rate of a stream whose SPS declares none, or overrides it. NACK is negotiated, RTX is not, so a retransmission arrives on the original sequence space and the loss a viewer reports is what was never recovered.

**Loss and retransmission.** A viewer counts loss from sequence numbers (`internal/rtpstats`): expected is the span from the first extended sequence number to the highest, lost is expected minus the distinct packets received. A packet missing at one moment and retransmitted later, after the viewer's NACK, arrives on its original sequence number, is marked received, and stops counting as lost — the counter has no state "lost, then recovered", so the `lost` a report carries is what never arrived by the end of the viewer's run, and recovery shows instead in `nacksSent` and in `lateCompletedFrames`. `tooLate` is not where a recovered packet goes: it counts a packet more than 65,535 sequence numbers behind the highest, too old to tell from a duplicate, which a retransmission never is; a second copy of a packet already received is a duplicate. The 50-viewer runs in [`evals/2026-10-05-mediamtx-one-way-delay.md`](evals/2026-10-05-mediamtx-one-way-delay.md) reported 3 and 18 packets lost with no incomplete frame. By this definition those packets never arrived at all, so they must sit in frames the reassembler does not count — a viewer's first frame, whose head it cannot prove, or the frames still pending when it stopped — and those reports do not say which; nor do they carry `nacksSent`, which a run from now on does. Jitter takes every received packet in arrival order, a retransmission included, so recovered loss raises it.

## Latency, and its limits

No figure whipbench reports is called "latency": the word covers too many different numbers to rank two servers by. [WB-1](BACKLOG.md) decided which ones whipbench measures; this section says what each is and what it is not.

**Packet transit — what 0.0.1 reports.** The publisher writes the wall-clock time each RTP packet is handed to the stack into the **abs-capture-time** header extension (`http://www.webrtc.org/experiments/rtp-hdrext/abs-capture-time`, a 64-bit NTP timestamp; pion implements its payload as `rtp.AbsCaptureTimeExtension`). A viewer subtracts that stamp from the arrival time of the same packet. With a pre-encoded clip the moment of capture is the moment of sending, which is why the field carries the send time.

What that number is, and what it is not:

- It is **network plus server forwarding** plus both clients' WebRTC stacks — the delay a server adds to a stream it relays. It is **not glass-to-glass**: there is no camera, no encoder, no decoder, no jitter buffer and no display in the path.
- Both clocks must agree. On one host they are the same clock. Across machines they must be synchronised (NTP to a few milliseconds, PTP better), and the error of that synchronisation is the error of the result. `view --clock-peer` measures the offset between two hosts ([WB-3](BACKLOG.md)), but packet transit does not apply it: the figure stays as measured.
- It needs the server to negotiate the extension on both legs and forward it unchanged. A server that does not is reported as **packet transit: unavailable**, with the reason — not negotiated, no stamps arrived, or stamps that give negative or implausible delays. whipbench never substitutes an estimate.
- Many viewers on one machine load that machine's CPU, and a starved reader adds delay on the client side. Watch the client in a large run; the report records the client's OS, architecture and CPU count.

**One-way delay — the headline, built in `run` ([WB-38](BACKLOG.md)).** Per frame: from the publisher writing the frame's first packet to the viewer receiving its last, one sample per complete frame. It is still network plus server forwarding, still not glass-to-glass, and it is the figure servers are ranked by. Its send instant has two sources, side by side and never averaged: the **fingerprint** — each viewer hashes the frame it reassembled and looks up when the publisher sent those bytes, which needs no header extension and so works through a server like MediaMTX ([WB-38](BACKLOG.md), the figure v0.1.0's report is built on) — and the abs-capture-time **stamp**, once per frame instead of per packet, which replaces packet transit ([WB-39](BACKLOG.md)). Every figure carries its topology, clock method, uncertainty and whether it can be compared with another report ([WB-40](BACKLOG.md), below under Reports); its sample window and what sits beside it are [WB-41](BACKLOG.md)'s, below. Between two hosts [WB-3](BACKLOG.md)'s exchange measures the clock offset, which no figure applies yet — `view` has no send log, so its fingerprint block is unavailable — and WB-39's stamp source will be the first to use it. The report key `oneWayDelay` is a list of source blocks, per viewer and pooled: `source`, `frameEnd`, `completeFrames`, `incompleteFrames`, `samples`, `invalid`, `unmatchedFrames`, `uncertaintyMs`, `comparable` and `notComparableReason`, `excludedFrames` and `lateCompletedFrames`, and the pooled block adds `viewers`, `viewersByFrameEnd`, `duplicateFrames`, `loopFrames` and `loopMinMs`. A frame still incomplete 1 s after its first packet is counted and never hashed; a viewer's first frame and the frames pending when it stops are not counted. A delay longer than `loopMinMs` is caught only by the RTP timestamp check, and a viewer's first match is taken as it is. There is no Prometheus series yet ([WB-42](BACKLOG.md)), and the CPU cost per viewer is unmeasured ([WB-25](BACKLOG.md)).

**Capture-to-decode — later, in v0.6.0.** A frame index drawn into the clips and read back from decoded VP8 keyframes on a sample of viewers ([WB-2](BACKLOG.md)). It differs from one-way delay only where a server transcodes. A timestamp drawn live into the frames would need a live encoder, which whipbench does not have.

## Scenarios

A scenario is a declarative JSON file, in the spirit of a k6 options block. Unknown fields are an error, so a typo cannot fall back to a default:

```json
{
  "name": "mediamtx-local-50",
  "whip": "http://127.0.0.1:8889/bench/whip",
  "whep": "http://127.0.0.1:8889/bench/whep",
  "codec": "vp8",
  "viewers": 50,
  "rampSeconds": 10,
  "holdSeconds": 30,
  "warmupSeconds": 2,
  "joinTimeoutSeconds": 10,
  "excludeFirstSeconds": 5,
  "includeLoopback": true,
  "metrics": "127.0.0.1:9464"
}
```

The publisher connects, the warmup passes, viewer *i* of *n* starts at *i*·ramp/*n*, and all of them stop together `holdSeconds` after the ramp ends. `bearerEnv` names an environment variable holding a bearer token; the token is sent and never written anywhere. Leave `whip` out to watch a stream something else publishes. `excludeFirstSeconds` (default 5) leaves each viewer's first seconds out of the one-way delay samples, counted from its own first RTP packet ([WB-41](BACKLOG.md)); the report records it in its scenario, and two reports that differ on it are not ranked against each other.

The ramp is deterministic. When its step is a multiple of the clip's 1 s GOP, every viewer arrives at the same point of the GOP and the join times cluster (ten viewers over ten seconds all wait about one second); a step that is not, such as 50 viewers over 10 s, samples the GOP evenly.

To sample the GOP evenly whatever the step, set `"rampOffsetSeed"` to any integer ([WB-8](BACKLOG.md)). Each viewer's start is then delayed by its own offset, drawn uniformly from [0, `rampOffsetMaxSeconds`) — 1 s by default, the clip's GOP — and the hold begins once that window has passed too, so the run lasts `rampSeconds` + `rampOffsetMaxSeconds` + `holdSeconds` after the warmup. The offsets come from SplitMix64, written out in `internal/scenario` rather than taken from Go's `math/rand`, so the same seed gives the same offsets on any machine and Go version. The report records the seed and the bound in its scenario and each viewer's offset as `rampOffsetMs`; its `startOffsetMs` is the ramp slot plus that offset. On the command line, `--ramp-offset-seed N` and `--ramp-offset-max 1s` set the two keys for `view`, and override the scenario's for `run`. It is off by default: without the seed the ramp is exactly the deterministic one above, and the report has neither key.

## Reports

Each run writes JSON (schema `whipbench.report/v0`) and a Markdown rendering of it. A report records the whipbench version, the scenario, its topology and clock, the publisher's figures, the client's own CPU and goroutines, every viewer's figures, the aggregates, error counts by kind, a per-second timeline and the definitions above.

- **Hosts only.** An endpoint is recorded as its host and port. Paths, query strings, user info and tokens are where servers carry stream keys, so they never reach a report, an error message or the console — the tests assert it.
- **No-verdict rule.** When more than 10% of the viewers failed to join, the aggregate is marked invalid and the Markdown prints the reason instead of the numbers. The viewers that did join describe a smaller run than the one asked for, and quoting them would flatter the server. A run whose publisher never streamed, or that was interrupted, has no verdict either.
- **Comparability.** Every report carries `topology` — `single-process` when the process that viewed also published (`run`), `split` otherwise (`view`) — and `clock`: `method`, `offsetMs`, `uncertaintyMs`, `stepDetected`, and no host names. A `run` reads both ends on its own monotonic clock, so the method is `monotonic` with offset and uncertainty 0 by construction; `stepDetected` is set when, between two of the run's once-a-second observations, the wall-clock and the monotonic time elapsed differ by more than 0.1 ms plus what a 500 ppm slew explains — so a time daemon's steady correction (2.8 ppm on the laptop measured) is never taken for a step, and a jump of 0.6 ms or more within a second is. A `view` given `--clock-peer` measures the publisher's clock with [WB-3](BACKLOG.md)'s exchange, and its method is `exchange`: each point (`points`: `tS` seconds since the run's start, `offsetMs`, the publisher's clock minus the viewer's, and `rttMs`) is the best of 16 UDP probes 10 ms apart, the offset halfway through the round trip and known to within half of it, so `offsetMs` is the first point's offset and `uncertaintyMs` the largest `rttMs`/2; between points the offset is piecewise-linear. It reads wall clocks, so a detected step makes its figures not comparable. A point that gets no answer is skipped, and a run with none at all has method `none` and `reason` "clock peer did not answer". A `view` without `--clock-peer` has no measured publisher clock: its method is `none`, it records no offset and no uncertainty — never a 0 — and none of its figures is comparable. Each one-way delay block says `comparable`, and when it is not, `notComparableReason`; it is comparable only when it is available, the clock was measured, the uncertainty is 1 ms or less and, on a wall clock, no step was detected. Two reports rank on a source only when both blocks are comparable, both aggregates valid, and the clip (codec and loop length) and every scenario key the same but the endpoint hosts, the name, the bearer variable and the metrics address. Sources are never averaged, and there is no merged best source.
- **Client resources.** `client.resources` says what the whipbench process itself spent over the run ([WB-17](BACKLOG.md)): `cpuSeconds`, user plus system CPU time from start to end; `cpuUtilisationPercent`, 100 × `cpuSeconds` / (wall time × `client.cpus`), so 100% is every core busy for the whole run; and `peakGoroutines`, the most seen at the start, once a second while viewers run and at the end. They describe the publisher, the viewers and the reassembly together, not the server; a client near its ceiling delays and drops packets itself. Where the platform gives no CPU reading (anything but linux and darwin) both CPU keys are absent and `cpuReason` says why. The Markdown prints them as a `client resources` row.
- **Metrics.** With `--metrics 127.0.0.1:9464` (or `"metrics"` in the scenario) the run serves Prometheus text format at `/metrics`: viewers started, joined, failed and active, packets and bytes received and sent, a histogram of packet transit (`whipbench_packet_transit_seconds`), and one of one-way delay labelled by the source of its send instant, `whipbench_one_way_delay_seconds{source="fingerprint"}` ([WB-42](BACKLOG.md)), so a long run shows the headline while it runs. Each viewer observes a sample exactly when its report block takes one — never an invalid or unmatched frame — on buckets from 0.1 ms to 1 s; there is no per-viewer label. Only a run that publishes has a send log, so a `view` declares the series with no sample. WB-39's stamp will be a second `source` value on the same name; an unlabelled one-way delay series is refused by the tests, because it would change meaning when that source lands. Three series describe the client rather than the server ([WB-17](BACKLOG.md)), read when `/metrics` is scraped: `whipbench_client_cpu_seconds_total`, the process's user plus system CPU time (getrusage, on linux and darwin; elsewhere the counter is declared with no sample, never a 0), `whipbench_client_goroutines` and `whipbench_client_heap_bytes`, the heap in use read through `runtime/metrics`, which does not stop the world. They cover the whole process — publisher, viewers and reassembly — so a client that has run out of CPU is visible before its figures are taken for the server's. [examples/grafana/](examples/grafana/) has a dashboard over these series for Grafana 10 or later, and how to scrape a run; a test fails when it queries a name `/metrics` does not serve. Per-layer series wait for simulcast ([WB-15](BACKLOG.md)). The exposition is written by hand — a dozen series did not justify the client library's dependency tree.

## First live numbers

[`evals/2026-10-05-mediamtx-one-way-delay.md`](evals/2026-10-05-mediamtx-one-way-delay.md): the same three scenarios with one-way delay by frame fingerprint (WB-5) — the first figure the report carries against a real server. p50 2.5 ms with 10 viewers, 4.4 ms (VP8) and 5.1 ms (H.264) with 50, p99 9.6–16.3 ms; every complete frame sampled, every report comparable, no clock step. Client and server shared one laptop, so the growth with the viewer count is not attributed to either.

[`evals/2026-10-01-mediamtx-local.md`](evals/2026-10-01-mediamtx-local.md): MediaMTX v1.21.1 in Docker on the same laptop, 10 and 50 viewers, VP8 and H.264. All viewers joined, no packet was lost, and packet transit (the 0.0.1 reports' `latency` key) was unavailable because MediaMTX's answers did not negotiate abs-capture-time. One machine, one server, loopback: it shows the tool works end to end and nothing about how MediaMTX compares with anything.

[`evals/2026-10-04-server-forwarding.md`](evals/2026-10-04-server-forwarding.md): what four servers forward (WB-4). No server checked — MediaMTX, OvenMediaEngine, Janus — negotiates abs-capture-time on either leg, so one-way delay against them comes from the frame fingerprint alone. Janus 1.1.2 behind Meetecho's WHIP and WHEP servers forwards every frame byte for byte with its marker bit, as MediaMTX does; OvenMediaEngine v0.21.0 and LiveKit cannot be viewed over WHEP. The Janus setup is in that directory, reproducible.

## How it compares

Two open tools sit closest. Both are good at what they set out to do; whipbench sets out to do something narrower.

- **[Softvelum whep-load-tester](https://github.com/Softvelum/whep-load-tester)** (Go, pion, MIT) opens many native WHEP playback sessions to find a server's playback capacity. It does not publish over WHIP, and its documentation does not describe a latency measurement.
- **[webrtcperf](https://github.com/vpalmisano/webrtcperf)** (Node.js, AGPL) drives real headless Chromium instances through Puppeteer and collects the browser's own statistics, including latency from a timestamp watermark on the frames and from abs-capture-time. A real browser is the most faithful client there is, and costs a browser per viewer; WHIP and WHEP are not what it is built around.

whipbench is the combination neither aims at: both standard endpoints, native clients cheap enough to run hundreds from one machine, and one report format with fixed definitions — including a no-verdict rule and an explicit "unavailable" — so that the same scenario against any server that speaks WHIP and WHEP produces results that can be put side by side. That is a real limit: of the four servers first planned, MediaMTX and Janus (behind Meetecho's WHIP and WHEP servers) qualify, while OvenMediaEngine has no WHEP and LiveKit has WHIP only through a transcoding ingress ([`evals/2026-10-04-server-forwarding.md`](evals/2026-10-04-server-forwarding.md)).

## Limits

Stated plainly, because a benchmark that hides them is worse than none:

- **Not glass-to-glass**, and no visual quality: the viewers never decode. "First keyframe" is the moment a frame could be decoded.
- **The clip cannot answer PLI or FIR.** A new viewer waits for the next keyframe of the loop, up to 1 s, so join time measures the server plus up to one GOP. A server that caches the last keyframe will look faster here than one that does not, which is a real difference, but a different one from a live encoder that can be asked for a keyframe.
- **Video only, one stream, no simulcast** yet ([WB-15](BACKLOG.md)), no audio, no TURN, host ICE candidates only (no STUN), no trickle ICE: each offer is sent after gathering completes.
- **Packet transit needs the extension** to survive the server, and clocks that agree; otherwise it is unavailable.
- **One-way delay needs the publisher in the same process** (`run`), and the cost of hashing every frame on every viewer is not yet measured.
- **One client machine** loads its own CPU and network; at high viewer counts the client can become the bottleneck before the server does.

## Load-testing etiquette

Run whipbench against servers you operate, or against a managed service only on your own account and within its terms, or with the written permission of whoever runs it. A load test against someone else's service is indistinguishable from an attack on it. The live numbers in this repository come from local servers only.

A run against a managed service records, beside the server version and host, the plan the account is on, the region, and the permission it ran under — own account and which terms, or who gave written permission and for what. No managed-service number is published without them. The longer form, with what to check before such a run, is [docs/load-testing-etiquette.md](docs/load-testing-etiquette.md) ([WB-6](BACKLOG.md)). It is not legal advice and does not interpret any provider's terms.

## Development

```bash
go vet ./... && go test -race ./...     # unit tests and real round trips through an in-process relay
golangci-lint run ./...
./scripts/check-repo.sh                 # version, changelog and README invariants
node scripts/leakcheck.mjs              # nothing private in the tracked files or the history
npm run backlog && npm run build:site   # backlog lint, roadmap freshness, the Pages site
```

The end-to-end tests need no network and no external server: `internal/testserver` is a small WHIP/WHEP relay on pion, run on loopback inside the test, that forwards the publisher's packets to every viewer and can strip the header extension, refuse viewers past a limit or drop packets. [CONTRIBUTING.md](CONTRIBUTING.md) has the conventions and the release runbook.

## Roadmap

[BACKLOG.md](BACKLOG.md) is the plan and [ROADMAP.md](ROADMAP.md) is generated from it. In short: v0.1.0 measures one-way delay by frame fingerprint (WB-1 decided it, WB-38 builds it) and records a live run against MediaMTX with the first report; v0.2.0 adds simulcast, layer switches and metrics, and the stamp, clock and comparability around one-way delay (WB-39 to WB-41); v0.3.0 is the comparative report across four servers and a write-up; v0.4.0 spreads the load over several machines and reports the client's own ceiling; v0.5.0 runs whipbench in CI with assertions and a GitHub Action; v0.6.0 measures what the viewer sees — capture-to-decode (WB-2), freezes, picture quality, impaired networks; and v1.0.0 freezes the report schema, the CLI and the scenario keys, with every published number reproducible.

## License

MIT — see [LICENSE](LICENSE).
