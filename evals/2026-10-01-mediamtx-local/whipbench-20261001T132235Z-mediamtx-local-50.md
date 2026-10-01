# mediamtx-local-50

| | |
|---|---|
| whipbench | v0.0.0-20261001132140-1455cf3b72b9 |
| started | 2026-10-01 13:22:35Z |
| duration | 42.3 s |
| WHIP host | `127.0.0.1:8889` |
| WHEP host | `127.0.0.1:8889` |
| scenario | 50 viewers, ramp 10s, hold 30s, warmup 2s, join timeout 10s, codec vp8 |
| client | darwin/arm64, 10 CPUs, go1.27.1 |

## Verdict

**valid: 50 of 50 viewers joined.**

50 viewers asked for, 50 joined, 0 failed (0%), 0 dropped after joining.

## Aggregate

| metric | n | p50 | p95 | p99 | min | max |
|---|---:|---:|---:|---:|---:|---:|
| join: first keyframe | 50 | 624.5 ms | 1225.1 ms | 1405.4 ms | 201.0 ms | 1405.4 ms |
| join: first RTP packet | 50 | 233.3 ms | 239.1 ms | 434.1 ms | 30.3 ms | 434.1 ms |
| signalling (POST → answer) | 50 | 7.4 ms | 13.0 ms | 22.6 ms | 1.9 ms | 22.6 ms |
| loss per viewer | 50 | 0.00% | 0.00% | 0.00% | 0.00% | 0.00% |
| jitter per viewer | 50 | 1.9 ms | 2.2 ms | 2.5 ms | 1.2 ms | 2.5 ms |
| keyframe interval per viewer | 50 | 1.00 s | 1.00 s | 1.00 s | 1.00 s | 1.00 s |

**Latency: unavailable** — not negotiated: the WHEP answer did not accept abs-capture-time.

Packets: 120663 received, 0 lost (0.000% of expected), 0 stalls across all viewers.

## Publisher

codec vp8, connected in 218 ms, 1262 frames and 2892 packets sent over 42.0 s (10 loops), schedule slips 0, send-time stamp: no — the WHIP answer did not accept abs-capture-time.

## Viewers

| id | start | joined | first RTP | first keyframe | received | lost | loss | jitter | keyframe | stalls | latency p50 | latency p99 | error |
|---:|---:|:---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| 0 | 0 ms | yes | 34 ms | 1001 ms | 2755 | 0 | 0.00% | 2.20 ms | 1.00 s | 0 | n/a | n/a |  |
| 1 | 200 ms | yes | 235 ms | 801 ms | 2733 | 0 | 0.00% | 1.52 ms | 1.00 s | 0 | n/a | n/a |  |
| 2 | 400 ms | yes | 233 ms | 601 ms | 2720 | 0 | 0.00% | 1.98 ms | 1.00 s | 0 | n/a | n/a |  |
| 3 | 600 ms | yes | 233 ms | 401 ms | 2707 | 0 | 0.00% | 1.74 ms | 1.00 s | 0 | n/a | n/a |  |
| 4 | 800 ms | yes | 33 ms | 201 ms | 2707 | 0 | 0.00% | 1.92 ms | 1.00 s | 0 | n/a | n/a |  |
| 5 | 1000 ms | yes | 235 ms | 1006 ms | 2673 | 0 | 0.00% | 2.44 ms | 1.00 s | 0 | n/a | n/a |  |
| 6 | 1200 ms | yes | 34 ms | 804 ms | 2676 | 0 | 0.00% | 1.91 ms | 1.00 s | 0 | n/a | n/a |  |
| 7 | 1400 ms | yes | 234 ms | 604 ms | 2649 | 0 | 0.00% | 1.46 ms | 1.00 s | 0 | n/a | n/a |  |
| 8 | 1600 ms | yes | 434 ms | 1405 ms | 2613 | 0 | 0.00% | 1.99 ms | 1.00 s | 0 | n/a | n/a |  |
| 9 | 1800 ms | yes | 34 ms | 205 ms | 2636 | 0 | 0.00% | 2.12 ms | 1.00 s | 0 | n/a | n/a |  |
| 10 | 2000 ms | yes | 235 ms | 1007 ms | 2605 | 0 | 0.00% | 1.90 ms | 1.00 s | 0 | n/a | n/a |  |
| 11 | 2200 ms | yes | 236 ms | 807 ms | 2593 | 0 | 0.00% | 1.37 ms | 1.00 s | 0 | n/a | n/a |  |
| 12 | 2400 ms | yes | 233 ms | 606 ms | 2580 | 0 | 0.00% | 1.67 ms | 1.00 s | 0 | n/a | n/a |  |
| 13 | 2600 ms | yes | 232 ms | 406 ms | 2568 | 0 | 0.00% | 1.71 ms | 1.00 s | 0 | n/a | n/a |  |
| 14 | 2800 ms | yes | 235 ms | 1206 ms | 2551 | 0 | 0.00% | 1.85 ms | 1.00 s | 0 | n/a | n/a |  |
| 15 | 3000 ms | yes | 236 ms | 1005 ms | 2539 | 0 | 0.00% | 1.89 ms | 1.00 s | 0 | n/a | n/a |  |
| 16 | 3200 ms | yes | 35 ms | 807 ms | 2539 | 0 | 0.00% | 2.09 ms | 1.00 s | 0 | n/a | n/a |  |
| 17 | 3400 ms | yes | 34 ms | 605 ms | 2527 | 0 | 0.00% | 1.80 ms | 1.00 s | 0 | n/a | n/a |  |
| 18 | 3600 ms | yes | 236 ms | 407 ms | 2502 | 0 | 0.00% | 1.94 ms | 1.00 s | 0 | n/a | n/a |  |
| 19 | 3800 ms | yes | 34 ms | 205 ms | 2502 | 0 | 0.00% | 1.86 ms | 1.00 s | 0 | n/a | n/a |  |
| 20 | 4000 ms | yes | 35 ms | 1008 ms | 2484 | 0 | 0.00% | 1.51 ms | 1.00 s | 0 | n/a | n/a |  |
| 21 | 4200 ms | yes | 35 ms | 806 ms | 2471 | 0 | 0.00% | 1.68 ms | 1.00 s | 0 | n/a | n/a |  |
| 22 | 4400 ms | yes | 30 ms | 601 ms | 2457 | 0 | 0.00% | 1.59 ms | 1.00 s | 0 | n/a | n/a |  |
| 23 | 4600 ms | yes | 234 ms | 403 ms | 2431 | 0 | 0.00% | 1.89 ms | 1.00 s | 0 | n/a | n/a |  |
| 24 | 4800 ms | yes | 234 ms | 1225 ms | 2413 | 0 | 0.00% | 1.76 ms | 1.00 s | 0 | n/a | n/a |  |
| 25 | 5000 ms | yes | 34 ms | 1025 ms | 2412 | 0 | 0.00% | 1.97 ms | 1.00 s | 0 | n/a | n/a |  |
| 26 | 5200 ms | yes | 237 ms | 825 ms | 2387 | 0 | 0.00% | 2.14 ms | 1.00 s | 0 | n/a | n/a |  |
| 27 | 5400 ms | yes | 236 ms | 622 ms | 2370 | 0 | 0.00% | 2.51 ms | 1.00 s | 0 | n/a | n/a |  |
| 28 | 5600 ms | yes | 36 ms | 422 ms | 2373 | 0 | 0.00% | 2.19 ms | 1.00 s | 0 | n/a | n/a |  |
| 29 | 5800 ms | yes | 34 ms | 218 ms | 2360 | 0 | 0.00% | 1.66 ms | 1.00 s | 0 | n/a | n/a |  |
| 30 | 6000 ms | yes | 68 ms | 1006 ms | 2336 | 0 | 0.00% | 1.88 ms | 1.00 s | 0 | n/a | n/a |  |
| 31 | 6200 ms | yes | 235 ms | 808 ms | 2317 | 0 | 0.00% | 1.97 ms | 1.00 s | 0 | n/a | n/a |  |
| 32 | 6400 ms | yes | 36 ms | 608 ms | 2317 | 0 | 0.00% | 1.82 ms | 1.00 s | 0 | n/a | n/a |  |
| 33 | 6600 ms | yes | 237 ms | 406 ms | 2292 | 0 | 0.00% | 2.02 ms | 1.00 s | 0 | n/a | n/a |  |
| 34 | 6800 ms | yes | 237 ms | 1209 ms | 2275 | 0 | 0.00% | 1.49 ms | 1.00 s | 0 | n/a | n/a |  |
| 35 | 7000 ms | yes | 233 ms | 1010 ms | 2263 | 0 | 0.00% | 1.79 ms | 1.00 s | 0 | n/a | n/a |  |
| 36 | 7200 ms | yes | 34 ms | 809 ms | 2263 | 0 | 0.00% | 1.63 ms | 1.00 s | 0 | n/a | n/a |  |
| 37 | 7400 ms | yes | 35 ms | 607 ms | 2248 | 0 | 0.00% | 2.11 ms | 1.00 s | 0 | n/a | n/a |  |
| 38 | 7600 ms | yes | 32 ms | 408 ms | 2238 | 0 | 0.00% | 1.76 ms | 1.00 s | 0 | n/a | n/a |  |
| 39 | 7800 ms | yes | 36 ms | 208 ms | 2226 | 0 | 0.00% | 1.80 ms | 1.00 s | 0 | n/a | n/a |  |
| 40 | 8000 ms | yes | 237 ms | 1013 ms | 2195 | 0 | 0.00% | 1.94 ms | 1.00 s | 0 | n/a | n/a |  |
| 41 | 8200 ms | yes | 239 ms | 812 ms | 2181 | 0 | 0.00% | 1.74 ms | 1.00 s | 0 | n/a | n/a |  |
| 42 | 8400 ms | yes | 36 ms | 607 ms | 2181 | 0 | 0.00% | 2.02 ms | 1.00 s | 0 | n/a | n/a |  |
| 43 | 8600 ms | yes | 39 ms | 410 ms | 2168 | 0 | 0.00% | 1.86 ms | 1.00 s | 0 | n/a | n/a |  |
| 44 | 8800 ms | yes | 235 ms | 1227 ms | 2137 | 0 | 0.00% | 1.79 ms | 1.00 s | 0 | n/a | n/a |  |
| 45 | 9000 ms | yes | 239 ms | 1024 ms | 2124 | 0 | 0.00% | 1.79 ms | 1.00 s | 0 | n/a | n/a |  |
| 46 | 9200 ms | yes | 242 ms | 821 ms | 2111 | 0 | 0.00% | 1.41 ms | 1.00 s | 0 | n/a | n/a |  |
| 47 | 9400 ms | yes | 237 ms | 625 ms | 2090 | 0 | 0.00% | 1.19 ms | 1.00 s | 0 | n/a | n/a |  |
| 48 | 9600 ms | yes | 237 ms | 411 ms | 2084 | 0 | 0.00% | 1.53 ms | 1.00 s | 0 | n/a | n/a |  |
| 49 | 9800 ms | yes | 38 ms | 225 ms | 2084 | 0 | 0.00% | 2.03 ms | 1.00 s | 0 | n/a | n/a |  |

## Method

- Join time is measured from the WHEP POST (after host-candidate ICE gathering). firstRtp: the first RTP packet arrives. firstKeyframe: the last packet of the first keyframe received complete arrives — no decoder runs, so this is when a frame could first be decoded, not when one was. A viewer has joined when it reaches firstKeyframe within the join timeout.
- Loss: expected = highest − first extended sequence number + 1 (RFC 3550 A.1), lost = expected − received, duplicates not counted. Measured on the stream the viewer receives, after NACK recovery; RTX is not negotiated, so a retransmission counts as received.
- Jitter: RFC 3550 §6.4.1 interarrival jitter, J += (\|D\| − J)/16 per packet, in milliseconds.
- Keyframe interval: spacing of keyframe starts in RTP time, i.e. the GOP the server delivers. The publisher's clip has a fixed 1 s GOP and cannot answer PLI, so a joining viewer waits for the next keyframe in the loop.
- Latency: the publisher stamps each packet's wall-clock send time in the abs-capture-time RTP header extension; a viewer's sample is its arrival time minus the stamp. It is network plus server forwarding plus both clients' stacks, on one host or synchronised clocks — not glass-to-glass. When the server does not negotiate or forward the extension, latency is reported unavailable, never estimated.
- Percentiles are nearest-rank. Join, loss and jitter summaries take one value per joined viewer; latency pools every valid sample of every viewer with latency, in a histogram with 1% buckets.
- No-verdict rule: when more than 10% of the viewers failed to join, the aggregate is not valid and must not be quoted.
