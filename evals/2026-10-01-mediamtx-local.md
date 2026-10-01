# 2026-10-01 — MediaMTX, local, 10 and 50 viewers

The first live run: whipbench against a real server rather than its own test relay.
It shows that the publisher, the viewers and the report work end to end against a
server whipbench did not write. It says nothing about how MediaMTX compares with any
other server — that needs the same scenarios against the others, on the same machine
(WB-20).

## Setup

| | |
|---|---|
| server | MediaMTX **v1.21.1** (`bluenviron/mediamtx:latest`, linux/arm64), default configuration plus `MTX_WEBRTCADDITIONALHOSTS=127.0.0.1` |
| server host | `127.0.0.1:8889` (WHIP and WHEP), ICE on `127.0.0.1:8189/udp`, published from Docker on the same machine |
| client | whipbench `v0.0.0-20261001132140-1455cf3b72b9` (the commit before this file), darwin/arm64, 10 CPUs, go1.27.1 |
| network | loopback into Docker Desktop's VM; nothing left the machine |
| clip | the embedded clip: 640×360, 30 fps, 1 s GOP, about 600 kbit/s |
| scenarios | [`examples/`](../examples/): ramp 10 s, hold 30 s, warmup 2 s, join timeout 10 s |

```bash
docker run --rm -d --name mediamtx -e MTX_WEBRTCADDITIONALHOSTS=127.0.0.1 \
  -p 127.0.0.1:8889:8889 -p 127.0.0.1:8189:8189/udp bluenviron/mediamtx:latest
whipbench run examples/mediamtx-local-10.json      --out evals/2026-10-01-mediamtx-local/
whipbench run examples/mediamtx-local-50.json      --out evals/2026-10-01-mediamtx-local/
whipbench run examples/mediamtx-local-50-h264.json --out evals/2026-10-01-mediamtx-local/
```

Each run exited 0 (valid). The raw reports, JSON and Markdown, are in
[`2026-10-01-mediamtx-local/`](2026-10-01-mediamtx-local/).

## Results

| run | joined | join, first keyframe p50 / p95 / max | join, first RTP p50 / p95 | signalling p50 / p95 | jitter p50 / max | loss | stalls |
|---|---:|---:|---:|---:|---:|---:|---:|
| VP8, 10 viewers | 10 / 10 | 1004 / 1014 / 1014 ms | 67 / 239 ms | 7 / 18 ms | 0.66 / 0.79 ms | 0 of 24,406 | 0 |
| VP8, 50 viewers | 50 / 50 | 625 / 1225 / 1405 ms | 233 / 239 ms | 7 / 13 ms | 1.85 / 2.51 ms | 0 of 120,663 | 0 |
| H.264, 50 viewers | 50 / 50 | 609 / 1017 / 1206 ms | 36 / 238 ms | 7 / 17 ms | 1.48 / 2.77 ms | 0 of 158,027 | 0 |

Every viewer saw a keyframe interval of exactly 1.00 s in RTP time — MediaMTX passes
the GOP through unchanged. No viewer dropped after joining, the publisher had no
schedule slip, and no error was counted. With 50 viewers, p99 is the 50th value, so it
equals the maximum; it is left out of the table for that reason.

**Latency: unavailable.** MediaMTX's WHIP and WHEP answers do not negotiate
abs-capture-time, so the publisher sent no stamp and every viewer reported "not
negotiated". Checked directly: whipbench's WHEP offer carries `abs-capture-time`, `mid`,
`rtp-stream-id`, `repaired-rtp-stream-id` and transport-wide-cc; MediaMTX's answer
keeps the last four and drops `abs-capture-time`. That finding opens WB-4's table.

## What the numbers do and do not show

- **Join time is mostly GOP wait.** The clip cannot answer a keyframe request, and
  MediaMTX did not hand a joining viewer a cached keyframe, so a viewer waits for the
  next one in the loop: anything up to 1 s on top of signalling and ICE. With ten
  viewers over ten seconds the ramp step equals the GOP, every viewer arrived at the same
  point of it, and all ten waited about a second (min 1000 ms) — an artefact of the
  scenario, not of the server, now WB-8. The 50-viewer ramps (200 ms step) sample the
  GOP evenly, and their median sits near the 0.5 s the GOP alone predicts plus about
  100–200 ms of connection set-up.
- **First RTP is bimodal** — about 35 ms or about 235 ms after the POST, the second
  cluster presumably an ICE connectivity-check retry. Not investigated.
- **Jitter grows with the viewer count** (0.7 ms → 2 ms at the median). The client
  machine runs all 50 viewers and the server in one VM; this run cannot say which side
  the extra jitter comes from.
- **Zero loss over loopback is expected** and is not evidence about MediaMTX on a real
  network.
- One machine, one run per scenario, no repetition: no confidence interval is implied.
