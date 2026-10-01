# mediamtx-local-50-h264

| | |
|---|---|
| whipbench | v0.0.0-20261001132140-1455cf3b72b9 |
| started | 2026-10-01 13:23:20Z |
| duration | 42.1 s |
| WHIP host | `127.0.0.1:8889` |
| WHEP host | `127.0.0.1:8889` |
| scenario | 50 viewers, ramp 10s, hold 30s, warmup 2s, join timeout 10s, codec h264 |
| client | darwin/arm64, 10 CPUs, go1.27.1 |

## Verdict

**valid: 50 of 50 viewers joined.**

50 viewers asked for, 50 joined, 0 failed (0%), 0 dropped after joining.

## Aggregate

| metric | n | p50 | p95 | p99 | min | max |
|---|---:|---:|---:|---:|---:|---:|
| join: first keyframe | 50 | 609.3 ms | 1017.1 ms | 1205.8 ms | 198.0 ms | 1205.8 ms |
| join: first RTP packet | 50 | 36.0 ms | 237.6 ms | 271.3 ms | 31.7 ms | 271.3 ms |
| signalling (POST → answer) | 50 | 7.1 ms | 16.8 ms | 31.7 ms | 3.1 ms | 31.7 ms |
| loss per viewer | 50 | 0.00% | 0.00% | 0.00% | 0.00% | 0.00% |
| jitter per viewer | 50 | 1.5 ms | 2.4 ms | 2.8 ms | 0.7 ms | 2.8 ms |
| keyframe interval per viewer | 50 | 1.00 s | 1.00 s | 1.00 s | 1.00 s | 1.00 s |

**Latency: unavailable** — not negotiated: the WHEP answer did not accept abs-capture-time.

Packets: 158027 received, 0 lost (0.000% of expected), 0 stalls across all viewers.

## Publisher

codec h264, connected in 15 ms, 1262 frames and 3770 packets sent over 42.0 s (10 loops), schedule slips 0, send-time stamp: no — the WHIP answer did not accept abs-capture-time.

## Viewers

| id | start | joined | first RTP | first keyframe | received | lost | loss | jitter | keyframe | stalls | latency p50 | latency p99 | error |
|---:|---:|:---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| 0 | 0 ms | yes | 234 ms | 1001 ms | 3593 | 0 | 0.00% | 1.05 ms | 1.00 s | 0 | n/a | n/a |  |
| 1 | 200 ms | yes | 32 ms | 798 ms | 3593 | 0 | 0.00% | 0.98 ms | 1.00 s | 0 | n/a | n/a |  |
| 2 | 400 ms | yes | 32 ms | 599 ms | 3575 | 0 | 0.00% | 1.09 ms | 1.00 s | 0 | n/a | n/a |  |
| 3 | 600 ms | yes | 32 ms | 399 ms | 3557 | 0 | 0.00% | 1.68 ms | 1.00 s | 0 | n/a | n/a |  |
| 4 | 800 ms | yes | 32 ms | 198 ms | 3539 | 0 | 0.00% | 0.75 ms | 1.00 s | 0 | n/a | n/a |  |
| 5 | 1000 ms | yes | 234 ms | 1002 ms | 3497 | 0 | 0.00% | 1.94 ms | 1.00 s | 0 | n/a | n/a |  |
| 6 | 1200 ms | yes | 34 ms | 803 ms | 3498 | 0 | 0.00% | 1.48 ms | 1.00 s | 0 | n/a | n/a |  |
| 7 | 1400 ms | yes | 33 ms | 600 ms | 3480 | 0 | 0.00% | 1.74 ms | 1.00 s | 0 | n/a | n/a |  |
| 8 | 1600 ms | yes | 33 ms | 401 ms | 3462 | 0 | 0.00% | 1.17 ms | 1.00 s | 0 | n/a | n/a |  |
| 9 | 1800 ms | yes | 32 ms | 201 ms | 3444 | 0 | 0.00% | 1.49 ms | 1.00 s | 0 | n/a | n/a |  |
| 10 | 2000 ms | yes | 235 ms | 1005 ms | 3408 | 0 | 0.00% | 0.93 ms | 1.00 s | 0 | n/a | n/a |  |
| 11 | 2200 ms | yes | 232 ms | 804 ms | 3385 | 0 | 0.00% | 2.77 ms | 1.00 s | 0 | n/a | n/a |  |
| 12 | 2400 ms | yes | 32 ms | 601 ms | 3393 | 0 | 0.00% | 0.72 ms | 1.00 s | 0 | n/a | n/a |  |
| 13 | 2600 ms | yes | 35 ms | 405 ms | 3375 | 0 | 0.00% | 1.10 ms | 1.00 s | 0 | n/a | n/a |  |
| 14 | 2800 ms | yes | 37 ms | 204 ms | 3357 | 0 | 0.00% | 1.50 ms | 1.00 s | 0 | n/a | n/a |  |
| 15 | 3000 ms | yes | 35 ms | 1012 ms | 3335 | 0 | 0.00% | 1.94 ms | 1.00 s | 0 | n/a | n/a |  |
| 16 | 3200 ms | yes | 35 ms | 814 ms | 3322 | 0 | 0.00% | 1.81 ms | 1.00 s | 0 | n/a | n/a |  |
| 17 | 3400 ms | yes | 235 ms | 613 ms | 3284 | 0 | 0.00% | 2.35 ms | 1.00 s | 0 | n/a | n/a |  |
| 18 | 3600 ms | yes | 36 ms | 414 ms | 3291 | 0 | 0.00% | 0.99 ms | 1.00 s | 0 | n/a | n/a |  |
| 19 | 3800 ms | yes | 34 ms | 213 ms | 3273 | 0 | 0.00% | 1.15 ms | 1.00 s | 0 | n/a | n/a |  |
| 20 | 4000 ms | yes | 35 ms | 1007 ms | 3242 | 0 | 0.00% | 1.65 ms | 1.00 s | 0 | n/a | n/a |  |
| 21 | 4200 ms | yes | 35 ms | 807 ms | 3232 | 0 | 0.00% | 1.70 ms | 1.00 s | 0 | n/a | n/a |  |
| 22 | 4400 ms | yes | 237 ms | 610 ms | 3196 | 0 | 0.00% | 1.57 ms | 1.00 s | 0 | n/a | n/a |  |
| 23 | 4600 ms | yes | 235 ms | 408 ms | 3178 | 0 | 0.00% | 1.65 ms | 1.00 s | 0 | n/a | n/a |  |
| 24 | 4800 ms | yes | 34 ms | 206 ms | 3178 | 0 | 0.00% | 1.55 ms | 1.00 s | 0 | n/a | n/a |  |
| 25 | 5000 ms | yes | 237 ms | 1032 ms | 3129 | 0 | 0.00% | 2.72 ms | 1.00 s | 0 | n/a | n/a |  |
| 26 | 5200 ms | yes | 37 ms | 827 ms | 3137 | 0 | 0.00% | 1.82 ms | 1.00 s | 0 | n/a | n/a |  |
| 27 | 5400 ms | yes | 38 ms | 630 ms | 3119 | 0 | 0.00% | 1.60 ms | 1.00 s | 0 | n/a | n/a |  |
| 28 | 5600 ms | yes | 37 ms | 433 ms | 3101 | 0 | 0.00% | 1.48 ms | 1.00 s | 0 | n/a | n/a |  |
| 29 | 5800 ms | yes | 38 ms | 234 ms | 3083 | 0 | 0.00% | 1.76 ms | 1.00 s | 0 | n/a | n/a |  |
| 30 | 6000 ms | yes | 271 ms | 1011 ms | 3044 | 0 | 0.00% | 1.13 ms | 1.00 s | 0 | n/a | n/a |  |
| 31 | 6200 ms | yes | 235 ms | 810 ms | 3024 | 0 | 0.00% | 2.27 ms | 1.00 s | 0 | n/a | n/a |  |
| 32 | 6400 ms | yes | 34 ms | 604 ms | 3032 | 0 | 0.00% | 1.74 ms | 1.00 s | 0 | n/a | n/a |  |
| 33 | 6600 ms | yes | 36 ms | 412 ms | 3014 | 0 | 0.00% | 1.45 ms | 1.00 s | 0 | n/a | n/a |  |
| 34 | 6800 ms | yes | 236 ms | 1206 ms | 2974 | 0 | 0.00% | 1.59 ms | 1.00 s | 0 | n/a | n/a |  |
| 35 | 7000 ms | yes | 238 ms | 1009 ms | 2961 | 0 | 0.00% | 1.08 ms | 1.00 s | 0 | n/a | n/a |  |
| 36 | 7200 ms | yes | 33 ms | 806 ms | 2956 | 0 | 0.00% | 2.28 ms | 1.00 s | 0 | n/a | n/a |  |
| 37 | 7400 ms | yes | 34 ms | 609 ms | 2947 | 0 | 0.00% | 1.34 ms | 1.00 s | 0 | n/a | n/a |  |
| 38 | 7600 ms | yes | 234 ms | 410 ms | 2912 | 0 | 0.00% | 0.94 ms | 1.00 s | 0 | n/a | n/a |  |
| 39 | 7800 ms | yes | 35 ms | 208 ms | 2912 | 0 | 0.00% | 1.39 ms | 1.00 s | 0 | n/a | n/a |  |
| 40 | 8000 ms | yes | 238 ms | 1010 ms | 2871 | 0 | 0.00% | 0.81 ms | 1.00 s | 0 | n/a | n/a |  |
| 41 | 8200 ms | yes | 34 ms | 809 ms | 2871 | 0 | 0.00% | 1.82 ms | 1.00 s | 0 | n/a | n/a |  |
| 42 | 8400 ms | yes | 37 ms | 609 ms | 2853 | 0 | 0.00% | 0.91 ms | 1.00 s | 0 | n/a | n/a |  |
| 43 | 8600 ms | yes | 33 ms | 415 ms | 2835 | 0 | 0.00% | 0.98 ms | 1.00 s | 0 | n/a | n/a |  |
| 44 | 8800 ms | yes | 39 ms | 204 ms | 2817 | 0 | 0.00% | 0.67 ms | 1.00 s | 0 | n/a | n/a |  |
| 45 | 9000 ms | yes | 237 ms | 1017 ms | 2770 | 0 | 0.00% | 2.23 ms | 1.00 s | 0 | n/a | n/a |  |
| 46 | 9200 ms | yes | 235 ms | 812 ms | 2758 | 0 | 0.00% | 1.20 ms | 1.00 s | 0 | n/a | n/a |  |
| 47 | 9400 ms | yes | 38 ms | 614 ms | 2758 | 0 | 0.00% | 0.78 ms | 1.00 s | 0 | n/a | n/a |  |
| 48 | 9600 ms | yes | 36 ms | 411 ms | 2740 | 0 | 0.00% | 1.71 ms | 1.00 s | 0 | n/a | n/a |  |
| 49 | 9800 ms | yes | 35 ms | 214 ms | 2722 | 0 | 0.00% | 0.81 ms | 1.00 s | 0 | n/a | n/a |  |

## Method

- Join time is measured from the WHEP POST (after host-candidate ICE gathering). firstRtp: the first RTP packet arrives. firstKeyframe: the last packet of the first keyframe received complete arrives — no decoder runs, so this is when a frame could first be decoded, not when one was. A viewer has joined when it reaches firstKeyframe within the join timeout.
- Loss: expected = highest − first extended sequence number + 1 (RFC 3550 A.1), lost = expected − received, duplicates not counted. Measured on the stream the viewer receives, after NACK recovery; RTX is not negotiated, so a retransmission counts as received.
- Jitter: RFC 3550 §6.4.1 interarrival jitter, J += (\|D\| − J)/16 per packet, in milliseconds.
- Keyframe interval: spacing of keyframe starts in RTP time, i.e. the GOP the server delivers. The publisher's clip has a fixed 1 s GOP and cannot answer PLI, so a joining viewer waits for the next keyframe in the loop.
- Latency: the publisher stamps each packet's wall-clock send time in the abs-capture-time RTP header extension; a viewer's sample is its arrival time minus the stamp. It is network plus server forwarding plus both clients' stacks, on one host or synchronised clocks — not glass-to-glass. When the server does not negotiate or forward the extension, latency is reported unavailable, never estimated.
- Percentiles are nearest-rank. Join, loss and jitter summaries take one value per joined viewer; latency pools every valid sample of every viewer with latency, in a histogram with 1% buckets.
- No-verdict rule: when more than 10% of the viewers failed to join, the aggregate is not valid and must not be quoted.
