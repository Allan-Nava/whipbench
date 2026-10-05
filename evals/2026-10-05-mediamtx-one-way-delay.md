# 2026-10-05 — MediaMTX, local: the first one-way delay figure (WB-5)

The 0.0.1 smoke run (`2026-10-01-mediamtx-local.md`) repeated with the method WB-1 decided:
one-way delay per frame, by frame fingerprint (WB-38), first-packet send to last-packet
arrival, on the one process's monotonic clock. Same scenarios, same server version, same
machine. It is the first report that carries a one-way delay figure. Like the smoke run, it
says nothing about how MediaMTX compares with another server.

## Setup

| | |
|---|---|
| server | MediaMTX **v1.21.1** (`bluenviron/mediamtx:1.21.1`, linux/arm64), default configuration plus `MTX_WEBRTCADDITIONALHOSTS=127.0.0.1` |
| server host | `127.0.0.1:8889` (WHIP and WHEP), ICE on `127.0.0.1:8189/udp`, Docker on the same machine |
| client | whipbench `v0.0.0-20261004213113-28dcd491dcce` (main at `28dcd49`), darwin/arm64, 10 CPUs, go1.27.1 |
| machine | one laptop on battery, kept awake for the runs (`caffeinate -dimu`); no sleep or wake was logged between 09:43:45 and 09:45:52 |
| network | loopback into Docker Desktop's VM; nothing left the machine |
| clip | the embedded clip: 640×360, 30 fps, 1 s GOP, 120 frames, every frame's bytes distinct |
| scenarios | [`examples/`](../examples/): ramp 10 s, hold 30 s, warmup 2 s, join timeout 10 s |

```bash
docker run --rm -d --name mediamtx -e MTX_WEBRTCADDITIONALHOSTS=127.0.0.1 \
  -p 127.0.0.1:8889:8889 -p 127.0.0.1:8189:8189/udp bluenviron/mediamtx:1.21.1
whipbench run examples/mediamtx-local-10.json      --out evals/2026-10-05-mediamtx-one-way-delay/
whipbench run examples/mediamtx-local-50.json      --out evals/2026-10-05-mediamtx-one-way-delay/
whipbench run examples/mediamtx-local-50-h264.json --out evals/2026-10-05-mediamtx-one-way-delay/
```

Each run exited 0 (valid) in 42 s. Reports, JSON and Markdown:
[`2026-10-05-mediamtx-one-way-delay/`](2026-10-05-mediamtx-one-way-delay/).

## One-way delay

Every report says `topology: single-process`, clock `monotonic` with offset and uncertainty 0
by construction, `stepDetected: false`, and the fingerprint block `comparable: true`.

| run | samples | p50 | p95 | p99 | min | max |
|---|---:|---:|---:|---:|---:|---:|
| VP8, 10 viewers | 10,640 | 2.5 ms | 5.2 ms | 9.6 ms | 0.6 ms | 25.3 ms |
| VP8, 50 viewers | 52,597 | 4.4 ms | 9.7 ms | 15.9 ms | 0.9 ms | 34.0 ms |
| H.264, 50 viewers | 52,565 | 5.1 ms | 10.1 ms | 16.3 ms | 0.8 ms | 26.5 ms |

The accounting behind each figure, the same on all three runs: every complete frame was
sampled — 0 incomplete, 0 unmatched, 0 invalid, 0 clip duplicates — and every viewer found
each frame's end by its marker bit. The publisher had no schedule slip; the fastest loop of
120 frames was sent in 3971 ms (10 viewers) and 3997–3998 ms (50).

Values are bucketed at ±0.5 % and printed to 0.1 ms (WB-1, D7).

## Everything else, beside the 0.0.1 run

| run | joined | join, first keyframe p50 / p95 / max | join, first RTP p50 / p95 | signalling p50 / p95 | jitter p50 / max | lost | stalls |
|---|---:|---:|---:|---:|---:|---:|---:|
| VP8, 10 viewers | 10 / 10 | 1001 / 1007 / 1007 ms | 33 / 34 ms | 6 / 13 ms | 0.46 / 0.50 ms | 0 of 24,467 | 0 |
| VP8, 50 viewers | 50 / 50 | 603 / 1010 / 1017 ms | 34 / 234 ms | 5 / 10 ms | 2.11 / 3.66 ms | 3 of 121,069 | 0 |
| H.264, 50 viewers | 50 / 50 | 606 / 1014 / 1210 ms | 34 / 66 ms | 6 / 12 ms | 1.31 / 3.65 ms | 18 of 158,203 | 0 |

Join, signalling and jitter sit where the 0.0.1 run put them. The first RTP packet is still
sometimes about 200 ms late (the second cluster the smoke run saw, not investigated). Packet
transit is still **unavailable**: MediaMTX's answers do not negotiate abs-capture-time
(`2026-10-04-server-forwarding.md`). No viewer dropped after joining, and no error was counted.

## What the numbers do and do not show

- **The delay grows with the viewer count**, 2.5 ms → 4.4 ms at the median and 9.6 → 15.9 ms
  at p99, as jitter does. The client runs the publisher, all 50 viewers' reassembly and
  hashing, and Docker's VM with the server, on one machine: this run cannot say how much of
  the growth is the server's and how much the client's own load. WB-25 (the client's own
  ceiling) is what would separate them.
- **The figure is end to end on one clock, not a network latency.** It includes the
  publisher's pacing and send path, MediaMTX's forwarding, loopback into the VM and back, and
  the viewer's reassembly up to the frame's last packet. Over loopback the network share is
  close to nothing, which is why this is a statement about the tool and the server together.
- **A few packets lost, no frame lost.** Three and eighteen packets were reported lost on the
  50-viewer runs, yet no frame stayed incomplete: NACK is negotiated, and the reassembler's
  1 s window lets a retransmission complete a frame. How the loss counter relates to packets
  recovered late is WB-41's open question; it is not reconciled here.
- **The p50 differs from the first attempt** (2.1 / 4.4 / 4.3 ms on 2026-10-04, at `b6149e9`,
  before the step-check fix, reports not kept) by up to 0.8 ms. One run per scenario, on a
  laptop: no confidence interval is implied, and two runs of the same scenario disagree at
  that level.
- An earlier attempt the same night is discarded: the machine slept mid-run and one scenario
  took 30 minutes. Nothing from it is used.

## Not covered

One server, one machine, one run per scenario, loopback, no loss or impairment, no simulcast.
The split topology (`view` on another host) has no delay figure until WB-3 measures the
publisher's clock.
