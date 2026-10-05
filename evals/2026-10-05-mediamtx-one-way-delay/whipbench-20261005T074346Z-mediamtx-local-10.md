# mediamtx-local-10

| | |
|---|---|
| whipbench | v0.0.0-20261004213113-28dcd491dcce |
| started | 2026-10-05 07:43:46Z |
| duration | 42.0 s |
| WHIP host | `127.0.0.1:8889` |
| WHEP host | `127.0.0.1:8889` |
| scenario | 10 viewers, ramp 10s, hold 30s, warmup 2s, join timeout 10s, codec vp8 |
| client | darwin/arm64, 10 CPUs, go1.27.1 |
| topology | single-process |
| clock | monotonic, offset 0 ms ± 0 ms, no step |

## Verdict

**valid: 10 of 10 viewers joined.**

10 viewers asked for, 10 joined, 0 failed (0%), 0 dropped after joining.

## Aggregate

| metric | n | p50 | p95 | p99 | min | max |
|---|---:|---:|---:|---:|---:|---:|
| one-way delay (fingerprint, per frame) | 10640 | 2.5 ms | 5.2 ms | 9.6 ms | 0.6 ms | 25.3 ms |
| join: first keyframe | 10 | 1001.3 ms | 1006.6 ms | 1006.6 ms | 999.0 ms | 1006.6 ms |
| join: first RTP packet | 10 | 32.8 ms | 34.3 ms | 34.3 ms | 32.1 ms | 34.3 ms |
| signalling (POST → answer) | 10 | 6.4 ms | 13.1 ms | 13.1 ms | 4.4 ms | 13.1 ms |
| loss per viewer | 10 | 0.00% | 0.00% | 0.00% | 0.00% | 0.00% |
| jitter per viewer | 10 | 0.5 ms | 0.5 ms | 0.5 ms | 0.4 ms | 0.5 ms |
| keyframe interval per viewer | 10 | 1.00 s | 1.00 s | 1.00 s | 1.00 s | 1.00 s |

One-way delay (fingerprint): 10640 samples from 10640 complete frames on 10 viewers (frame end: marker 10); 0 incomplete, 0 unmatched, 0 invalid; 0 duplicate clip frames never sampled; loop 120 frames, sent in 3971 ms at the fastest.

**Packet transit: unavailable** — not negotiated: the WHEP answer did not accept abs-capture-time.

Packets: 24467 received, 0 lost (0.000% of expected), 0 stalls across all viewers.

## Publisher

codec vp8, connected in 23 ms, 1261 frames and 2890 packets sent over 42.0 s (10 loops), schedule slips 0, send-time stamp: no — the WHIP answer did not accept abs-capture-time.

## Viewers

| id | start | joined | first RTP | first keyframe | received | lost | loss | jitter | keyframe | stalls | delay p50 | delay p99 | transit p50 | transit p99 | error |
|---:|---:|:---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| 0 | 0 ms | yes | 33 ms | 1005 ms | 2760 | 0 | 0.00% | 0.48 ms | 1.00 s | 0 | 2.5 ms | 10.0 ms | n/a | n/a |  |
| 1 | 1000 ms | yes | 32 ms | 999 ms | 2689 | 0 | 0.00% | 0.45 ms | 1.00 s | 0 | 2.5 ms | 10.1 ms | n/a | n/a |  |
| 2 | 2000 ms | yes | 33 ms | 1002 ms | 2613 | 0 | 0.00% | 0.43 ms | 1.00 s | 0 | 2.5 ms | 9.5 ms | n/a | n/a |  |
| 3 | 3000 ms | yes | 32 ms | 1001 ms | 2551 | 0 | 0.00% | 0.50 ms | 1.00 s | 0 | 2.5 ms | 9.9 ms | n/a | n/a |  |
| 4 | 4000 ms | yes | 33 ms | 1000 ms | 2484 | 0 | 0.00% | 0.50 ms | 1.00 s | 0 | 2.5 ms | 9.9 ms | n/a | n/a |  |
| 5 | 5000 ms | yes | 33 ms | 1007 ms | 2413 | 0 | 0.00% | 0.46 ms | 1.00 s | 0 | 2.5 ms | 10.1 ms | n/a | n/a |  |
| 6 | 6000 ms | yes | 33 ms | 1002 ms | 2337 | 0 | 0.00% | 0.45 ms | 1.00 s | 0 | 2.5 ms | 8.8 ms | n/a | n/a |  |
| 7 | 7000 ms | yes | 33 ms | 1001 ms | 2275 | 0 | 0.00% | 0.48 ms | 1.00 s | 0 | 2.6 ms | 10.1 ms | n/a | n/a |  |
| 8 | 8000 ms | yes | 34 ms | 1003 ms | 2208 | 0 | 0.00% | 0.45 ms | 1.00 s | 0 | 2.6 ms | 9.0 ms | n/a | n/a |  |
| 9 | 9000 ms | yes | 33 ms | 1001 ms | 2137 | 0 | 0.00% | 0.47 ms | 1.00 s | 0 | 2.6 ms | 9.4 ms | n/a | n/a |  |

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
