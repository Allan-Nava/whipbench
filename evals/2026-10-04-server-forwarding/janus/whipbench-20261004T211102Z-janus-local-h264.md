# janus-local-h264

| | |
|---|---|
| whipbench | v0.0.0-20261004053617-5286f6189e94 |
| started | 2026-10-04 21:11:02Z |
| duration | 17.0 s |
| WHIP host | `wb-janus:7080` |
| WHEP host | `wb-janus:7090` |
| scenario | 1 viewers, ramp 0s, hold 15s, warmup 2s, join timeout 10s, codec h264 |
| client | linux/arm64, 5 CPUs, go1.27.1 |

## Verdict

**valid: 1 of 1 viewers joined.**

1 viewers asked for, 1 joined, 0 failed (0%), 0 dropped after joining.

## Aggregate

| metric | n | p50 | p95 | p99 | min | max |
|---|---:|---:|---:|---:|---:|---:|
| one-way delay (fingerprint, per frame) | 448 | 0.8 ms | 2.2 ms | 3.0 ms | 0.4 ms | 4.9 ms |
| join: first keyframe | 1 | 1000.3 ms | 1000.3 ms | 1000.3 ms | 1000.3 ms | 1000.3 ms |
| join: first RTP packet | 1 | 64.5 ms | 64.5 ms | 64.5 ms | 64.5 ms | 64.5 ms |
| signalling (POST → answer) | 1 | 9.2 ms | 9.2 ms | 9.2 ms | 9.2 ms | 9.2 ms |
| loss per viewer | 1 | 0.00% | 0.00% | 0.00% | 0.00% | 0.00% |
| jitter per viewer | 1 | 0.6 ms | 0.6 ms | 0.6 ms | 0.6 ms | 0.6 ms |
| keyframe interval per viewer | 1 | 1.00 s | 1.00 s | 1.00 s | 1.00 s | 1.00 s |

One-way delay (fingerprint): 448 samples from 448 complete frames on 1 viewers (frame end: marker 1); 0 incomplete, 0 unmatched, 0 invalid; 0 duplicate clip frames never sampled; loop 120 frames, sent in 3993 ms at the fastest.

**Packet transit: unavailable** — not negotiated: the WHEP answer did not accept abs-capture-time.

Packets: 1349 received, 0 lost (0.000% of expected), 0 stalls across all viewers.

## Publisher

codec h264, connected in 28 ms, 511 frames and 1529 packets sent over 17.0 s (4 loops), schedule slips 0, send-time stamp: no — the WHIP answer did not accept abs-capture-time.

## Viewers

| id | start | joined | first RTP | first keyframe | received | lost | loss | jitter | keyframe | stalls | delay p50 | delay p99 | transit p50 | transit p99 | error |
|---:|---:|:---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| 0 | 0 ms | yes | 64 ms | 1000 ms | 1349 | 0 | 0.00% | 0.62 ms | 1.00 s | 0 | 0.8 ms | 3.0 ms | n/a | n/a |  |

## Method

- Join time is measured from the WHEP POST (after host-candidate ICE gathering). firstRtp: the first RTP packet arrives. firstKeyframe: the last packet of the first keyframe received complete arrives — no decoder runs, so this is when a frame could first be decoded, not when one was. A viewer has joined when it reaches firstKeyframe within the join timeout.
- Loss: expected = highest − first extended sequence number + 1 (RFC 3550 A.1), lost = expected − received, duplicates not counted. Measured on the stream the viewer receives, after NACK recovery; RTX is not negotiated, so a retransmission counts as received.
- Jitter: RFC 3550 §6.4.1 interarrival jitter, J += (\|D\| − J)/16 per packet, in milliseconds.
- Keyframe interval: spacing of keyframe starts in RTP time, i.e. the GOP the server delivers. The publisher's clip has a fixed 1 s GOP and cannot answer PLI, so a joining viewer waits for the next keyframe in the loop.
- One-way delay (source fingerprint): per frame, first-packet send to last-packet arrival, on the monotonic clock of the one process that runs both ends. The publisher logs t0 just before it hands a frame's first packet to the stack, by absolute frame index. Each viewer reassembles frames by RTP timestamp; t1 is the arrival of the frame's last packet — the marker packet, or, on a stream without markers, the last before the next timestamp (frameEnd). A complete frame is hashed — the first 64 bits of SHA-256 over the whole VP8 frame, or over the H.264 VCL NAL units — and matched to the latest send of that clip frame at or before t1; the sample is t1 − t0. A frame still incomplete 1 s after its first packet counts in incompleteFrames and is never hashed; a viewer's first frame and the frames still pending when it stops are not counted. A frame the depacketiser rejects or that is not in the clip counts in unmatchedFrames; a frame whose bytes repeat in the clip (duplicateFrames) is never sampled; a match the RTP timestamps prove whole loops too new, or one with no logged send at or before t1, is invalid. loopMinMs is the shortest time the publisher took to send loopFrames frames: a delay longer than that is caught only by that RTP timestamp check, and a viewer's first match is taken as it is. Retransmitted packets count like any other. Network plus server forwarding plus both clients' stacks — not glass-to-glass; a `view` run has no send log and reports it unavailable. The CPU cost of reassembling and hashing every frame on every viewer is not measured.
- Packet transit: the publisher stamps each packet's wall-clock send time in the abs-capture-time RTP header extension; a viewer's sample is its arrival time minus the stamp. It is network plus server forwarding plus both clients' stacks, per packet rather than per frame, on one host or synchronised clocks — not glass-to-glass. When the server does not negotiate or forward the extension, packet transit is reported unavailable, never estimated.
- Percentiles are nearest-rank. Join, loss and jitter summaries take one value per joined viewer; packet transit pools every valid sample of every viewer that has it, in a histogram with 1% buckets.
- No-verdict rule: when more than 10% of the viewers failed to join, the aggregate is not valid and must not be quoted.
