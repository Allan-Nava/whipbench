# 00 · Questions — WB-38 One-way delay by frame fingerprint

**Written against:** `71a1be1`

The default assumption is what makes this phase non-blocking: work can proceed
without waiting for answers, and the assumptions are on the record.

---

## Ticket

**ID:** WB-38
**Link:** https://github.com/Allan-Nava/whipbench/blob/main/BACKLOG.md
**Title:** One-way delay by frame fingerprint

The headline delay figure, and the one WB-5 publishes (WB-1, D2 and D4 in
`thoughts/WB-1-latency-method/02-design.md`). In `run` the publisher logs t0 =
`time.Now()` just before each frame's first `WriteRTP`, under a 64-bit hash of the
frame's depacketised bytes: the whole VP8 frame, and for H.264 the VCL NAL units only,
because a server may add or repeat SPS/PPS. Every viewer reassembles frames (pion
`samplebuilder`), hashes them the same way and takes the latest send of that hash before
t1, the arrival of the frame's marker packet. One sample per complete frame,
NACK-recovered packets included; incomplete frames are counted, not sampled. Monotonic
clock at both ends, report keys under `oneWayDelay`, source `fingerprint`; no header
extension and no decoder, so it survives a server that drops abs-capture-time. The
report states its limit: a frame later than the 4 s clip loop is ambiguous, and its
sample is invalid. Byte-identical frames are excluded when the clip loads; a bitstream
rewrite counts as `unmatchedFrames`; a server that drops the marker bit (WB-4) ends the
frame at the last packet before the next RTP timestamp. Open: the CPU per viewer of
reassembly and hashing at scale is unmeasured (WB-25's ceiling).

The decision behind it is recorded in `thoughts/WB-1-latency-method/02-design.md` (D2,
D4) and in the Review comments there: hash the VCL NAL units only for H.264, and state
the 4 s clip-loop limit.

---

## Questions

### Q1 · How does a viewer tell a frame later than the 4 s clip loop from an on-time one, so that its sample is marked invalid rather than recorded?

- **Risk if unresolved:** every frame recurs once per loop, so "the latest send of that hash before t1" always finds a t0 less than 4 s old. A frame delayed 4.3 s matches the previous loop's send and reads as 0.3 s — a plausible, silently wrong headline number, the worst failure a benchmark can have. Stating the limit in the report does not by itself make any sample invalid.
- **Default assumption:** a sample counts as invalid only when the viewer has evidence of its own that the frame is more than one loop old; without such evidence the sample is kept, and the report states that delays of one loop or more alias to delay modulo the loop length. The loop length is the loaded clip's duration, read when the clip loads, not a hard-coded 4 s.
- **Answer:** default accepted (2026-10-03, Allan Nava, in chat: "accetta ipotesi").

### Q2 · Is WB-38 scoped to a single-process `run`, or must it also work when publisher and viewers run on different hosts?

- **Risk if unresolved:** the ticket says "in `run`" and "monotonic clock at both ends". Two hosts' monotonic clocks share no epoch, and the publisher's hash-to-t0 log has to reach the viewer somehow. If split runs are in scope, the task grows a transport for the log and a clock-offset dependency; if they are not, a split run must not print a number that looks comparable.
- **Default assumption:** single-process `run` only, where publisher and viewers share one monotonic clock and one in-memory log. A split run reports the fingerprint delay as unavailable, with the reason, and never a figure.
- **Answer:** default accepted (2026-10-03, Allan Nava, in chat: "accetta ipotesi").

### Q3 · Does the sample deliberately span first-packet send to last-packet (marker) arrival, so a large frame's own transmission time is part of its delay?

- **Risk if unresolved:** t0 is taken before the frame's first `WriteRTP` and t1 at its marker packet's arrival, so a keyframe of many packets carries its whole send and pacing time while a small delta frame does not. The distribution then mixes two populations, and two servers compared at different bitrates or GOPs differ for reasons that are not forwarding.
- **Default assumption:** yes, as written — one distribution over all complete frames, first-packet send to marker arrival. The report states this definition next to the figure; keyframes are not split out in this task.
- **Answer:** default accepted (2026-10-03, Allan Nava, in chat: "accetta ipotesi").

### Q4 · When is a frame complete — what is t1 for a frame finished by a NACK retransmission, and how long does a viewer wait before counting a frame incomplete?

- **Risk if unresolved:** if the marker packet arrives on time but a middle packet is recovered 200 ms later, taking t1 at the marker hides the retransmission delay the viewer actually suffered. The wait before giving up decides whether a late recovery becomes a tail sample or an incomplete count, so it moves p99 directly — and it is a parameter the reader cannot see.
- **Default assumption:** t1 is the arrival of the marker packet, as written, whether original or retransmitted; frames that needed a retransmission are counted beside the figure. A frame is incomplete when the reassembler gives it up, and that window is fixed and stated in the report.
- **Answer:** default accepted (2026-10-03, Allan Nava, in chat: "accetta ipotesi").

### Q5 · What does the report say when few or no frames match — and can it tell a server's bitstream rewrite apart from any other reason a frame went unmatched?

- **Risk if unresolved:** a server that re-packetises or transcodes yields `unmatchedFrames` for nearly every frame, and a delay computed from the handful that matched is biased yet still headlines. If a lost log entry, a hash computed differently, or a frame before the first keyframe also land in `unmatchedFrames`, the counter blames the server for a whipbench fault.
- **Default assumption:** the figure always carries its sample count and the `unmatchedFrames` count beside it; with zero matched frames the report says the delay is unavailable and why, never zero or blank. No minimum match fraction is imposed in this task. `unmatchedFrames` counts only complete frames whose hash is absent from the publisher's log.
- **Answer:** default accepted (2026-10-03, Allan Nava, in chat: "accetta ipotesi").

### Q6 · What happens to the existing packet-transit delay figure and its report keys once `oneWayDelay` carries the fingerprint figure?

- **Risk if unresolved:** if `oneWayDelay` today holds the 0.0.1 packet-transit figure, reusing the key with a new meaning breaks every report already published and any comparison across them; if both figures stay, a reader must know which one is the headline.
- **Default assumption:** `oneWayDelay` is the fingerprint figure with `source: fingerprint`; the packet-transit figure is kept unchanged under a separate, explicitly named key and is never the headline. Reports written before this change are not rewritten.
- **Answer:** default accepted (2026-10-03, Allan Nava, in chat: "accetta ipotesi").

### Q7 · What exactly is excluded as a "byte-identical frame" when the clip loads, and where are those frames counted at run time?

- **Risk if unresolved:** two identical frames in one loop share a hash, so a viewer cannot know which send it received; a 64-bit collision between different frames has the same effect. If exclusion means the frames are not sent, the published stream no longer matches the clip; if they are sent but not sampled and not counted, the sample count silently shrinks.
- **Default assumption:** every hash that occurs more than once in one loop of the clip is excluded from sampling; those frames are still sent, and their count is reported as a fixed property of the clip, separate from `unmatchedFrames` and from incomplete frames.
- **Answer:** default accepted (2026-10-03, Allan Nava, in chat: "accetta ipotesi").

### Q8 · For H.264, which bytes exactly make up "the VCL NAL units" that are hashed?

- **Risk if unresolved:** start codes or length prefixes, the NAL header byte, and SEI or AUD units a server may insert all change the hash. Hash too much and a server's harmless rewrite reads as `unmatchedFrames`; hash too little and the fingerprint stops being unique within a loop.
- **Default assumption:** the VCL NAL units of the access unit, in order, each including its one-byte NAL header and without start codes or length prefixes; every non-VCL unit (SPS, PPS, SEI, AUD) is skipped. Any change to those bytes by a server counts as a rewrite.
- **Answer:** default accepted (2026-10-03, Allan Nava, in chat: "accetta ipotesi").

### Q9 · Without a marker bit, is t1 the arrival of the frame's last packet or of the next frame's first packet, and is the fallback applied per stream or per frame?

- **Risk if unresolved:** a frame's end is only known once a packet with the next RTP timestamp arrives; taking that arrival as t1 adds up to one frame interval (about 33 ms at 30 fps) to every sample, against a server that keeps the marker. Switching rules frame by frame mixes two definitions in one distribution.
- **Default assumption:** t1 is the recorded arrival of the last packet carrying the frame's RTP timestamp, read back once the next timestamp shows up; a stream switches to the fallback when its packets stop carrying markers, and the report records which frame-end rule each viewer used.
- **Answer:** default accepted (2026-10-03, Allan Nava, in chat: "accetta ipotesi").

### Q10 · Is fingerprinting always on in `run`, or optional, given that its CPU cost per viewer at scale is unmeasured?

- **Risk if unresolved:** reassembly and hashing in every viewer take CPU from the load generator; at high viewer counts the measurement can lower the ceiling it sits beside, or skew the delay itself on a saturated host, and the report would not show it.
- **Default assumption:** always on in `run`, with no flag to turn it off; the report notes that its CPU cost is unmeasured. Measuring it is left to WB-25.
- **Answer:** default accepted (2026-10-03, Allan Nava, in chat: "accetta ipotesi").

---

## Out of scope

Things the ticket might suggest but that we are **not** doing in this task:

- Running the method against a live server and publishing the figure — that is WB-5.
- Verifying what each server forwards byte for byte, and whether it keeps the marker
  bit — that is WB-4; this task only handles both cases.
- Measuring the CPU cost of reassembly and hashing per viewer at scale — WB-25.
- Any RTP header extension as the delay stamp, and any decoder in the viewer.
- Glass-to-glass latency: the sample ends at frame arrival, not at display.
- Changing how the 4 s clip loop itself is built or lengthened.

---

## Status

- [x] Questions generated
- [x] Reviewed by a human (2026-10-03, Allan Nava)
- [x] Answers collected (or assumptions explicitly accepted) — all ten defaults accepted, 2026-10-03

> Next phase: **Research**. The ticket is **not** passed to Research — only the
> questions and their answers.
