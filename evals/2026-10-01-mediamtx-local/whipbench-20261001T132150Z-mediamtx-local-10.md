# mediamtx-local-10

| | |
|---|---|
| whipbench | v0.0.0-20261001132140-1455cf3b72b9 |
| started | 2026-10-01 13:21:50Z |
| duration | 42.0 s |
| WHIP host | `127.0.0.1:8889` |
| WHEP host | `127.0.0.1:8889` |
| scenario | 10 viewers, ramp 10s, hold 30s, warmup 2s, join timeout 10s, codec vp8 |
| client | darwin/arm64, 10 CPUs, go1.27.1 |

## Verdict

**valid: 10 of 10 viewers joined.**

10 viewers asked for, 10 joined, 0 failed (0%), 0 dropped after joining.

## Aggregate

| metric | n | p50 | p95 | p99 | min | max |
|---|---:|---:|---:|---:|---:|---:|
| join: first keyframe | 10 | 1003.6 ms | 1014.0 ms | 1014.0 ms | 999.9 ms | 1014.0 ms |
| join: first RTP packet | 10 | 67.4 ms | 239.2 ms | 239.2 ms | 33.2 ms | 239.2 ms |
| signalling (POST → answer) | 10 | 6.8 ms | 18.0 ms | 18.0 ms | 4.8 ms | 18.0 ms |
| loss per viewer | 10 | 0.00% | 0.00% | 0.00% | 0.00% | 0.00% |
| jitter per viewer | 10 | 0.7 ms | 0.8 ms | 0.8 ms | 0.5 ms | 0.8 ms |
| keyframe interval per viewer | 10 | 1.00 s | 1.00 s | 1.00 s | 1.00 s | 1.00 s |

**Latency: unavailable** — not negotiated: the WHEP answer did not accept abs-capture-time.

Packets: 24406 received, 0 lost (0.000% of expected), 0 stalls across all viewers.

## Publisher

codec vp8, connected in 15 ms, 1261 frames and 2890 packets sent over 42.0 s (10 loops), schedule slips 0, send-time stamp: no — the WHIP answer did not accept abs-capture-time.

## Viewers

| id | start | joined | first RTP | first keyframe | received | lost | loss | jitter | keyframe | stalls | latency p50 | latency p99 | error |
|---:|---:|:---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| 0 | 0 ms | yes | 239 ms | 1001 ms | 2747 | 0 | 0.00% | 0.71 ms | 1.00 s | 0 | n/a | n/a |  |
| 1 | 1000 ms | yes | 236 ms | 1004 ms | 2676 | 0 | 0.00% | 0.56 ms | 1.00 s | 0 | n/a | n/a |  |
| 2 | 2000 ms | yes | 236 ms | 1004 ms | 2605 | 0 | 0.00% | 0.74 ms | 1.00 s | 0 | n/a | n/a |  |
| 3 | 3000 ms | yes | 67 ms | 1000 ms | 2549 | 0 | 0.00% | 0.49 ms | 1.00 s | 0 | n/a | n/a |  |
| 4 | 4000 ms | yes | 235 ms | 1003 ms | 2471 | 0 | 0.00% | 0.61 ms | 1.00 s | 0 | n/a | n/a |  |
| 5 | 5000 ms | yes | 33 ms | 1006 ms | 2413 | 0 | 0.00% | 0.66 ms | 1.00 s | 0 | n/a | n/a |  |
| 6 | 6000 ms | yes | 34 ms | 1006 ms | 2337 | 0 | 0.00% | 0.79 ms | 1.00 s | 0 | n/a | n/a |  |
| 7 | 7000 ms | yes | 232 ms | 1007 ms | 2263 | 0 | 0.00% | 0.68 ms | 1.00 s | 0 | n/a | n/a |  |
| 8 | 8000 ms | yes | 34 ms | 1004 ms | 2208 | 0 | 0.00% | 0.70 ms | 1.00 s | 0 | n/a | n/a |  |
| 9 | 9000 ms | yes | 34 ms | 1014 ms | 2137 | 0 | 0.00% | 0.55 ms | 1.00 s | 0 | n/a | n/a |  |

## Method

- Join time is measured from the WHEP POST (after host-candidate ICE gathering). firstRtp: the first RTP packet arrives. firstKeyframe: the last packet of the first keyframe received complete arrives — no decoder runs, so this is when a frame could first be decoded, not when one was. A viewer has joined when it reaches firstKeyframe within the join timeout.
- Loss: expected = highest − first extended sequence number + 1 (RFC 3550 A.1), lost = expected − received, duplicates not counted. Measured on the stream the viewer receives, after NACK recovery; RTX is not negotiated, so a retransmission counts as received.
- Jitter: RFC 3550 §6.4.1 interarrival jitter, J += (\|D\| − J)/16 per packet, in milliseconds.
- Keyframe interval: spacing of keyframe starts in RTP time, i.e. the GOP the server delivers. The publisher's clip has a fixed 1 s GOP and cannot answer PLI, so a joining viewer waits for the next keyframe in the loop.
- Latency: the publisher stamps each packet's wall-clock send time in the abs-capture-time RTP header extension; a viewer's sample is its arrival time minus the stamp. It is network plus server forwarding plus both clients' stacks, on one host or synchronised clocks — not glass-to-glass. When the server does not negotiate or forward the extension, latency is reported unavailable, never estimated.
- Percentiles are nearest-rank. Join, loss and jitter summaries take one value per joined viewer; latency pools every valid sample of every viewer with latency, in a histogram with 1% buckets.
- No-verdict rule: when more than 10% of the viewers failed to join, the aggregate is not valid and must not be quoted.
