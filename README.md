# whipbench — a vendor-neutral WebRTC benchmark over WHIP and WHEP

<p align="center"><img src="https://raw.githubusercontent.com/Allan-Nava/whipbench/main/assets/logo.svg" width="72" height="72" alt=""></p>

whipbench publishes a synthetic clip over **WHIP** (RFC 9725), plays it back with any number of **WHEP** viewers, and writes a report of what an operator cares about — join time, one-way delay, packet loss, jitter, keyframe interval and run stability — with the same definitions whichever server sits in the middle. The clients are native Go on [pion/webrtc](https://github.com/pion/webrtc) v4, with no browser, so one machine can drive many viewers, and the report is meant to be laid next to another server's.

**Status: 0.0.1, not released.** The clients, the report and the in-process test relay work, and the first live run against MediaMTX is in [`evals/`](evals/). The latency method is the open question of the next milestone ([WB-1](BACKLOG.md)): MediaMTX does not negotiate the header extension the latency stamp travels in, so against it latency is reported **unavailable** — by design, never estimated.

## Install

From source, with Go 1.27 or later (pure Go, `CGO_ENABLED=0`):

```bash
go install github.com/Allan-Nava/whipbench/cmd/whipbench@latest
whipbench version
```

The two synthetic clips are embedded in the binary; nothing else has to sit beside it. Release binaries for Linux and macOS (amd64 and arm64) with checksums will be attached to each tagged release.

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

`run` and `view` print the verdict and write `whipbench-<UTC time>-<name>.json` and `.md`. The exit status is 0 for a valid run, 3 for a run with no verdict, 1 for an error and 2 for a usage error, so a CI job can gate on it.

## What it measures

Every viewer records its own numbers; the report adds aggregates over the viewers that joined. Definitions, because a benchmark number without one cannot be compared:

| measure | definition |
|---|---|
| join: first RTP packet | WHEP POST sent → first RTP packet received |
| join: first keyframe | WHEP POST sent → last packet of the first keyframe received **complete** (every sequence number from its first packet to its marker). No decoder runs, so this is the earliest moment a frame *could* be decoded, not a decoded frame. A viewer has **joined** when it gets here within the join timeout (10 s by default). |
| signalling | WHEP POST sent → SDP answer received |
| loss | expected = highest − first extended sequence number + 1 (RFC 3550 A.1); lost = expected − received; duplicates are not counted as received |
| jitter | RFC 3550 §6.4.1 interarrival jitter, J += (\|D\| − J)/16 on every packet, in milliseconds |
| keyframe interval | spacing of keyframe starts in RTP time — the GOP the server actually delivers |
| one-way delay | arrival time − the publisher's send-time stamp, per packet; see the next section |
| stability | stalls (a gap of 500 ms or more between packets), viewers dropped after joining, publisher schedule slips, and a per-second timeline of active viewers and packets |

Percentiles are nearest-rank: p95 is a value some viewer actually had. Join, loss and jitter take one value per joined viewer; one-way delay pools every packet of every viewer in a histogram with 1% buckets (min, max and mean exact).

The publisher streams a pre-encoded clip, 640×360 at 30 fps, 4 s, a keyframe every 30 frames, in VP8 or constrained-baseline H.264. [`scripts/make-clips.sh`](scripts/make-clips.sh) records the exact ffmpeg command and remakes them. Frames go on the wire byte for byte, paced at the frame rate, and the loop is seamless: frame *k* has RTP timestamp base + 3000·*k* whichever pass it belongs to. NACK is negotiated, RTX is not, so a retransmission arrives on the original sequence space and the loss a viewer reports is what was never recovered.

## Latency, and its limits

The publisher writes the wall-clock time each RTP packet is handed to the stack into the **abs-capture-time** header extension (`http://www.webrtc.org/experiments/rtp-hdrext/abs-capture-time`, a 64-bit NTP timestamp; pion implements its payload as `rtp.AbsCaptureTimeExtension`). A viewer subtracts that stamp from the arrival time of the same packet. With a pre-encoded clip the moment of capture is the moment of sending, which is why the field carries the send time.

What that number is, and what it is not:

- It is **network plus server forwarding** plus both clients' WebRTC stacks — the delay a server adds to a stream it relays. It is **not glass-to-glass**: there is no camera, no encoder, no decoder, no jitter buffer and no display in the path.
- Both clocks must agree. On one host they are the same clock. Across machines they must be synchronised (NTP to a few milliseconds, PTP better), and the error of that synchronisation is the error of the result. How to bound it is [WB-3](BACKLOG.md).
- It needs the server to negotiate the extension on both legs and forward it unchanged. A server that does not is reported as **latency: unavailable**, with the reason — not negotiated, no stamps arrived, or stamps that give negative or implausible delays. whipbench never substitutes an estimate.
- Many viewers on one machine load that machine's CPU, and a starved reader adds delay on the client side. Watch the client in a large run; the report records the client's OS, architecture and CPU count.

Measuring glass-to-glass through a timestamp drawn into the frames is the open design question of the next milestone ([WB-1](BACKLOG.md)), deliberately not built yet.

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
  "includeLoopback": true,
  "metrics": "127.0.0.1:9464"
}
```

The publisher connects, the warmup passes, viewer *i* of *n* starts at *i*·ramp/*n*, and all of them stop together `holdSeconds` after the ramp ends. `bearerEnv` names an environment variable holding a bearer token; the token is sent and never written anywhere. Leave `whip` out to watch a stream something else publishes.

The ramp is deterministic. When its step is a multiple of the clip's 1 s GOP, every viewer arrives at the same point of the GOP and the join times cluster (ten viewers over ten seconds all wait about one second); a step that is not, such as 50 viewers over 10 s, samples the GOP evenly.

## Reports

Each run writes JSON (schema `whipbench.report/v0`) and a Markdown rendering of it. A report records the whipbench version, the scenario, the publisher's figures, every viewer's figures, the aggregates, error counts by kind, a per-second timeline and the definitions above.

- **Hosts only.** An endpoint is recorded as its host and port. Paths, query strings, user info and tokens are where servers carry stream keys, so they never reach a report, an error message or the console — the tests assert it.
- **No-verdict rule.** When more than 10% of the viewers failed to join, the aggregate is marked invalid and the Markdown prints the reason instead of the numbers. The viewers that did join describe a smaller run than the one asked for, and quoting them would flatter the server. A run whose publisher never streamed, or that was interrupted, has no verdict either.
- **Metrics.** With `--metrics 127.0.0.1:9464` (or `"metrics"` in the scenario) the run serves Prometheus text format at `/metrics`: viewers started, joined, failed and active, packets and bytes received and sent, and a histogram of one-way delay. The exposition is written by hand — a dozen series did not justify the client library's dependency tree.

## First live numbers

[`evals/2026-10-01-mediamtx-local.md`](evals/2026-10-01-mediamtx-local.md): MediaMTX v1.21.1 in Docker on the same laptop, 10 and 50 viewers, VP8 and H.264. All viewers joined, no packet was lost, and latency was unavailable because MediaMTX's answers did not negotiate abs-capture-time. One machine, one server, loopback: it shows the tool works end to end and nothing about how MediaMTX compares with anything.

## How it compares

Two open tools sit closest. Both are good at what they set out to do; whipbench sets out to do something narrower.

- **[Softvelum whep-load-tester](https://github.com/Softvelum/whep-load-tester)** (Go, pion, MIT) opens many native WHEP playback sessions to find a server's playback capacity. It does not publish over WHIP, and its documentation does not describe a latency measurement.
- **[webrtcperf](https://github.com/vpalmisano/webrtcperf)** (Node.js, AGPL) drives real headless Chromium instances through Puppeteer and collects the browser's own statistics, including latency from a timestamp watermark on the frames and from abs-capture-time. A real browser is the most faithful client there is, and costs a browser per viewer; WHIP and WHEP are not what it is built around.

whipbench is the combination neither aims at: both standard endpoints, native clients cheap enough to run hundreds from one machine, and one report format with fixed definitions — including a no-verdict rule and an explicit "unavailable" — so that the same scenario against MediaMTX, OvenMediaEngine, LiveKit, Janus or a managed service produces results that can be put side by side.

## Limits

Stated plainly, because a benchmark that hides them is worse than none:

- **Not glass-to-glass**, and no visual quality: the viewers never decode. "First keyframe" is the moment a frame could be decoded.
- **The clip cannot answer PLI or FIR.** A new viewer waits for the next keyframe of the loop, up to 1 s, so join time measures the server plus up to one GOP. A server that caches the last keyframe will look faster here than one that does not, which is a real difference, but a different one from a live encoder that can be asked for a keyframe.
- **Video only, one stream, no simulcast** yet ([WB-15](BACKLOG.md)), no audio, no TURN, host ICE candidates only (no STUN), no trickle ICE: each offer is sent after gathering completes.
- **Latency needs the extension** to survive the server, and clocks that agree; otherwise it is unavailable.
- **One client machine** loads its own CPU and network; at high viewer counts the client can become the bottleneck before the server does.

## Load-testing etiquette

Run whipbench against servers you operate, or against a managed service only on your own account and within its terms, or with the written permission of whoever runs it. A load test against someone else's service is indistinguishable from an attack on it. The live numbers in this repository come from local servers only. The longer note is [WB-6](BACKLOG.md).

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

[BACKLOG.md](BACKLOG.md) is the plan and [ROADMAP.md](ROADMAP.md) is generated from it. In short: v0.1.0 decides the latency method (WB-1) and records a live run against MediaMTX with the first report; v0.2.0 adds simulcast, layer switches and metrics; v0.3.0 is the comparative report across four servers and a write-up; v0.4.0 spreads the load over several machines and reports the client's own ceiling; v0.5.0 runs whipbench in CI with assertions and a GitHub Action; v0.6.0 measures what the viewer sees — freezes, picture quality, impaired networks; and v1.0.0 freezes the report schema, the CLI and the scenario keys, with every published number reproducible.

## License

MIT — see [LICENSE](LICENSE).
