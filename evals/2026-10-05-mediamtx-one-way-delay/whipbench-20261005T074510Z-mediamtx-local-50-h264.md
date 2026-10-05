# mediamtx-local-50-h264

| | |
|---|---|
| whipbench | v0.0.0-20261004213113-28dcd491dcce |
| started | 2026-10-05 07:45:10Z |
| duration | 42.1 s |
| WHIP host | `127.0.0.1:8889` |
| WHEP host | `127.0.0.1:8889` |
| scenario | 50 viewers, ramp 10s, hold 30s, warmup 2s, join timeout 10s, codec h264 |
| client | darwin/arm64, 10 CPUs, go1.27.1 |
| topology | single-process |
| clock | monotonic, offset 0 ms ± 0 ms, no step |

## Verdict

**valid: 50 of 50 viewers joined.**

50 viewers asked for, 50 joined, 0 failed (0%), 0 dropped after joining.

## Aggregate

| metric | n | p50 | p95 | p99 | min | max |
|---|---:|---:|---:|---:|---:|---:|
| one-way delay (fingerprint, per frame) | 52565 | 5.1 ms | 10.1 ms | 16.3 ms | 0.8 ms | 26.5 ms |
| join: first keyframe | 50 | 606.2 ms | 1013.7 ms | 1210.1 ms | 199.2 ms | 1210.1 ms |
| join: first RTP packet | 50 | 33.7 ms | 66.1 ms | 633.5 ms | 31.8 ms | 633.5 ms |
| signalling (POST → answer) | 50 | 6.3 ms | 11.8 ms | 14.5 ms | 4.0 ms | 14.5 ms |
| loss per viewer | 50 | 0.00% | 0.07% | 0.09% | 0.00% | 0.09% |
| jitter per viewer | 50 | 1.3 ms | 3.2 ms | 3.6 ms | 0.6 ms | 3.6 ms |
| keyframe interval per viewer | 50 | 1.00 s | 1.00 s | 1.00 s | 1.00 s | 1.00 s |

One-way delay (fingerprint): 52565 samples from 52565 complete frames on 50 viewers (frame end: marker 50); 0 incomplete, 0 unmatched, 0 invalid; 0 duplicate clip frames never sampled; loop 120 frames, sent in 3997 ms at the fastest.

**Packet transit: unavailable** — not negotiated: the WHEP answer did not accept abs-capture-time.

Packets: 158185 received, 18 lost (0.011% of expected), 0 stalls across all viewers.

## Publisher

codec h264, connected in 9 ms, 1262 frames and 3770 packets sent over 42.0 s (10 loops), schedule slips 0, send-time stamp: no — the WHIP answer did not accept abs-capture-time.

## Viewers

| id | start | joined | first RTP | first keyframe | received | lost | loss | jitter | keyframe | stalls | delay p50 | delay p99 | transit p50 | transit p99 | error |
|---:|---:|:---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| 0 | 0 ms | yes | 33 ms | 1002 ms | 3610 | 0 | 0.00% | 0.67 ms | 1.00 s | 0 | 4.6 ms | 15.0 ms | n/a | n/a |  |
| 1 | 200 ms | yes | 33 ms | 804 ms | 3593 | 0 | 0.00% | 1.66 ms | 1.00 s | 0 | 4.7 ms | 15.6 ms | n/a | n/a |  |
| 2 | 400 ms | yes | 33 ms | 601 ms | 3569 | 0 | 0.00% | 2.46 ms | 1.00 s | 0 | 4.8 ms | 15.9 ms | n/a | n/a |  |
| 3 | 600 ms | yes | 33 ms | 403 ms | 3557 | 0 | 0.00% | 1.47 ms | 1.00 s | 0 | 5.0 ms | 16.6 ms | n/a | n/a |  |
| 4 | 800 ms | yes | 32 ms | 202 ms | 3539 | 0 | 0.00% | 1.51 ms | 1.00 s | 0 | 4.6 ms | 16.8 ms | n/a | n/a |  |
| 5 | 1000 ms | yes | 33 ms | 1001 ms | 3513 | 2 | 0.06% | 2.29 ms | 1.00 s | 0 | 4.9 ms | 16.8 ms | n/a | n/a |  |
| 6 | 1200 ms | yes | 33 ms | 801 ms | 3498 | 0 | 0.00% | 1.31 ms | 1.00 s | 0 | 4.8 ms | 14.7 ms | n/a | n/a |  |
| 7 | 1400 ms | yes | 33 ms | 602 ms | 3480 | 0 | 0.00% | 1.15 ms | 1.00 s | 0 | 4.8 ms | 16.4 ms | n/a | n/a |  |
| 8 | 1600 ms | yes | 33 ms | 404 ms | 3453 | 0 | 0.00% | 0.89 ms | 1.00 s | 0 | 4.8 ms | 16.6 ms | n/a | n/a |  |
| 9 | 1800 ms | yes | 33 ms | 201 ms | 3435 | 0 | 0.00% | 1.38 ms | 1.00 s | 0 | 5.0 ms | 14.0 ms | n/a | n/a |  |
| 10 | 2000 ms | yes | 34 ms | 1003 ms | 3412 | 0 | 0.00% | 1.42 ms | 1.00 s | 0 | 4.9 ms | 15.2 ms | n/a | n/a |  |
| 11 | 2200 ms | yes | 35 ms | 801 ms | 3404 | 0 | 0.00% | 0.61 ms | 1.00 s | 0 | 4.9 ms | 14.7 ms | n/a | n/a |  |
| 12 | 2400 ms | yes | 34 ms | 606 ms | 3393 | 0 | 0.00% | 1.00 ms | 1.00 s | 0 | 4.9 ms | 15.8 ms | n/a | n/a |  |
| 13 | 2600 ms | yes | 33 ms | 401 ms | 3377 | 0 | 0.00% | 2.03 ms | 1.00 s | 0 | 4.9 ms | 15.5 ms | n/a | n/a |  |
| 14 | 2800 ms | yes | 34 ms | 205 ms | 3357 | 0 | 0.00% | 0.80 ms | 1.00 s | 0 | 5.0 ms | 14.4 ms | n/a | n/a |  |
| 15 | 3000 ms | yes | 34 ms | 1005 ms | 3333 | 2 | 0.06% | 1.63 ms | 1.00 s | 0 | 5.0 ms | 15.5 ms | n/a | n/a |  |
| 16 | 3200 ms | yes | 32 ms | 800 ms | 3322 | 0 | 0.00% | 1.13 ms | 1.00 s | 0 | 4.9 ms | 16.1 ms | n/a | n/a |  |
| 17 | 3400 ms | yes | 32 ms | 605 ms | 3308 | 0 | 0.00% | 1.38 ms | 1.00 s | 0 | 5.0 ms | 17.4 ms | n/a | n/a |  |
| 18 | 3600 ms | yes | 32 ms | 405 ms | 3288 | 3 | 0.09% | 2.39 ms | 1.00 s | 0 | 4.9 ms | 15.8 ms | n/a | n/a |  |
| 19 | 3800 ms | yes | 32 ms | 199 ms | 3273 | 0 | 0.00% | 1.26 ms | 1.00 s | 0 | 5.1 ms | 16.3 ms | n/a | n/a |  |
| 20 | 4000 ms | yes | 32 ms | 1006 ms | 3249 | 0 | 0.00% | 0.80 ms | 1.00 s | 0 | 5.1 ms | 16.8 ms | n/a | n/a |  |
| 21 | 4200 ms | yes | 33 ms | 803 ms | 3223 | 0 | 0.00% | 1.02 ms | 1.00 s | 0 | 5.2 ms | 16.6 ms | n/a | n/a |  |
| 22 | 4400 ms | yes | 34 ms | 606 ms | 3205 | 0 | 0.00% | 0.73 ms | 1.00 s | 0 | 5.1 ms | 17.1 ms | n/a | n/a |  |
| 23 | 4600 ms | yes | 33 ms | 403 ms | 3187 | 0 | 0.00% | 1.21 ms | 1.00 s | 0 | 5.0 ms | 16.4 ms | n/a | n/a |  |
| 24 | 4800 ms | yes | 36 ms | 203 ms | 3178 | 0 | 0.00% | 0.86 ms | 1.00 s | 0 | 5.3 ms | 17.6 ms | n/a | n/a |  |
| 25 | 5000 ms | yes | 35 ms | 1012 ms | 3154 | 0 | 0.00% | 0.58 ms | 1.00 s | 0 | 5.3 ms | 16.1 ms | n/a | n/a |  |
| 26 | 5200 ms | yes | 33 ms | 805 ms | 3135 | 2 | 0.06% | 1.61 ms | 1.00 s | 0 | 5.4 ms | 16.4 ms | n/a | n/a |  |
| 27 | 5400 ms | yes | 234 ms | 609 ms | 3101 | 0 | 0.00% | 0.85 ms | 1.00 s | 0 | 5.2 ms | 16.6 ms | n/a | n/a |  |
| 28 | 5600 ms | yes | 32 ms | 408 ms | 3101 | 0 | 0.00% | 1.07 ms | 1.00 s | 0 | 5.2 ms | 16.6 ms | n/a | n/a |  |
| 29 | 5800 ms | yes | 35 ms | 207 ms | 3081 | 2 | 0.06% | 1.76 ms | 1.00 s | 0 | 5.1 ms | 16.9 ms | n/a | n/a |  |
| 30 | 6000 ms | yes | 36 ms | 1014 ms | 3060 | 0 | 0.00% | 0.99 ms | 1.00 s | 0 | 5.3 ms | 16.3 ms | n/a | n/a |  |
| 31 | 6200 ms | yes | 35 ms | 817 ms | 3047 | 0 | 0.00% | 1.32 ms | 1.00 s | 0 | 5.3 ms | 16.3 ms | n/a | n/a |  |
| 32 | 6400 ms | yes | 37 ms | 616 ms | 3034 | 0 | 0.00% | 2.73 ms | 1.00 s | 0 | 5.5 ms | 16.3 ms | n/a | n/a |  |
| 33 | 6600 ms | yes | 34 ms | 413 ms | 3014 | 0 | 0.00% | 0.89 ms | 1.00 s | 0 | 5.2 ms | 17.6 ms | n/a | n/a |  |
| 34 | 6800 ms | yes | 33 ms | 217 ms | 2987 | 0 | 0.00% | 1.29 ms | 1.00 s | 0 | 5.4 ms | 16.3 ms | n/a | n/a |  |
| 35 | 7000 ms | yes | 34 ms | 1011 ms | 2974 | 0 | 0.00% | 1.44 ms | 1.00 s | 0 | 5.3 ms | 15.5 ms | n/a | n/a |  |
| 36 | 7200 ms | yes | 34 ms | 808 ms | 2961 | 0 | 0.00% | 1.22 ms | 1.00 s | 0 | 5.4 ms | 17.3 ms | n/a | n/a |  |
| 37 | 7400 ms | yes | 33 ms | 609 ms | 2940 | 0 | 0.00% | 1.40 ms | 1.00 s | 0 | 5.4 ms | 16.6 ms | n/a | n/a |  |
| 38 | 7600 ms | yes | 39 ms | 415 ms | 2930 | 0 | 0.00% | 1.36 ms | 1.00 s | 0 | 5.1 ms | 15.6 ms | n/a | n/a |  |
| 39 | 7800 ms | yes | 36 ms | 210 ms | 2912 | 2 | 0.07% | 3.60 ms | 1.00 s | 0 | 5.2 ms | 15.2 ms | n/a | n/a |  |
| 40 | 8000 ms | yes | 36 ms | 1015 ms | 2889 | 1 | 0.03% | 3.05 ms | 1.00 s | 0 | 5.4 ms | 16.8 ms | n/a | n/a |  |
| 41 | 8200 ms | yes | 34 ms | 813 ms | 2871 | 2 | 0.07% | 3.65 ms | 1.00 s | 0 | 5.4 ms | 15.2 ms | n/a | n/a |  |
| 42 | 8400 ms | yes | 34 ms | 606 ms | 2855 | 0 | 0.00% | 2.71 ms | 1.00 s | 0 | 5.3 ms | 16.8 ms | n/a | n/a |  |
| 43 | 8600 ms | yes | 33 ms | 410 ms | 2837 | 0 | 0.00% | 2.39 ms | 1.00 s | 0 | 5.3 ms | 17.3 ms | n/a | n/a |  |
| 44 | 8800 ms | yes | 634 ms | 1210 ms | 2758 | 0 | 0.00% | 1.35 ms | 1.00 s | 0 | 5.4 ms | 16.8 ms | n/a | n/a |  |
| 45 | 9000 ms | yes | 35 ms | 1004 ms | 2793 | 0 | 0.00% | 0.74 ms | 1.00 s | 0 | 5.3 ms | 16.6 ms | n/a | n/a |  |
| 46 | 9200 ms | yes | 36 ms | 804 ms | 2776 | 0 | 0.00% | 0.82 ms | 1.00 s | 0 | 5.4 ms | 15.3 ms | n/a | n/a |  |
| 47 | 9400 ms | yes | 34 ms | 610 ms | 2760 | 0 | 0.00% | 1.29 ms | 1.00 s | 0 | 5.5 ms | 15.6 ms | n/a | n/a |  |
| 48 | 9600 ms | yes | 33 ms | 405 ms | 2740 | 2 | 0.07% | 3.18 ms | 1.00 s | 0 | 5.3 ms | 15.3 ms | n/a | n/a |  |
| 49 | 9800 ms | yes | 66 ms | 206 ms | 2719 | 0 | 0.00% | 1.04 ms | 1.00 s | 0 | 5.3 ms | 15.8 ms | n/a | n/a |  |

## Method

- Join time is measured from the WHEP POST (after host-candidate ICE gathering). firstRtp: the first RTP packet arrives. firstKeyframe: the last packet of the first keyframe received complete arrives — no decoder runs, so this is when a frame could first be decoded, not when one was. A viewer has joined when it reaches firstKeyframe within the join timeout.
- Loss: expected = highest − first extended sequence number + 1 (RFC 3550 A.1), lost = expected − received, duplicates not counted. Measured on the stream the viewer receives, after NACK recovery; RTX is not negotiated, so a retransmission counts as received.
- Jitter: RFC 3550 §6.4.1 interarrival jitter, J += (\|D\| − J)/16 per packet, in milliseconds.
- Keyframe interval: spacing of keyframe starts in RTP time, i.e. the GOP the server delivers. The publisher's clip has a fixed 1 s GOP and cannot answer PLI, so a joining viewer waits for the next keyframe in the loop.
- One-way delay (source fingerprint): per frame, first-packet send to last-packet arrival, on the monotonic clock of the one process that runs both ends. The publisher logs t0 just before it hands a frame's first packet to the stack, by absolute frame index. Each viewer reassembles frames by RTP timestamp; t1 is the arrival of the frame's last packet — the marker packet, or, on a stream without markers, the last before the next timestamp (frameEnd). A complete frame is hashed — the first 64 bits of SHA-256 over the whole VP8 frame, or over the H.264 VCL NAL units — and matched to the latest send of that clip frame at or before t1; the sample is t1 − t0. A frame still incomplete 1 s after its first packet counts in incompleteFrames and is never hashed; a viewer's first frame and the frames still pending when it stops are not counted. A frame the depacketiser rejects or that is not in the clip counts in unmatchedFrames; a frame whose bytes repeat in the clip (duplicateFrames) is never sampled; a match the RTP timestamps prove whole loops too new, or one with no logged send at or before t1, is invalid. loopMinMs is the shortest time the publisher took to send loopFrames frames: a delay longer than that is caught only by that RTP timestamp check, and a viewer's first match is taken as it is. Retransmitted packets count like any other. Network plus server forwarding plus both clients' stacks — not glass-to-glass; a `view` run has no send log and reports it unavailable. The CPU cost of reassembling and hashing every frame on every viewer is not measured.
- Comparability: topology is single-process when this process published and viewed, split otherwise. The clock block says how both ends share a time base — monotonic in a single-process run, offset 0 ± 0 ms by construction; none in a split run until the publisher's clock is measured, with no offset or uncertainty recorded — and stepDetected is set when wall-clock and monotonic elapsed time over the run differ by more than 0.1 ms. A source block is comparable only when it is available, the clock was measured, its uncertainty is 1 ms or less and, for a wall-clock method, no step was detected; an unavailable block carries no uncertainty. Two reports rank on a source only when both blocks are comparable, both aggregates valid, the clip (codec, loop frames) and every scenario key but the endpoint hosts the same. Sources are never averaged and there is no merged best source.
- Packet transit: the publisher stamps each packet's wall-clock send time in the abs-capture-time RTP header extension; a viewer's sample is its arrival time minus the stamp. It is network plus server forwarding plus both clients' stacks, per packet rather than per frame, on one host or synchronised clocks — not glass-to-glass. When the server does not negotiate or forward the extension, packet transit is reported unavailable, never estimated.
- Percentiles are nearest-rank. Join, loss and jitter summaries take one value per joined viewer; packet transit pools every valid sample of every viewer that has it, in a histogram with 1% buckets.
- No-verdict rule: when more than 10% of the viewers failed to join, the aggregate is not valid and must not be quoted.
