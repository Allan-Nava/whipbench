# 00 · Questions — WB-1 Latency method

**Written against:** `91bfe16`

The default assumption is what makes this phase non-blocking: work can proceed
without waiting for answers, and the assumptions are on the record.

---

## Ticket

**ID:** WB-1
**Link:** https://github.com/Allan-Nava/whipbench/blob/main/BACKLOG.md
**Title:** Latency method

whipbench 0.0.1 measures one-way delay by stamping each RTP packet's send time in the
abs-capture-time header extension and subtracting it at the viewer. That number is
network plus server forwarding, not glass-to-glass, needs one clock or synchronised
clocks, and is unavailable whenever a server does not negotiate or forward the
extension — the first server tried, MediaMTX v1.21.1, negotiates it on neither leg.
Decide how whipbench measures latency: whether abs-capture-time stays the network stamp
or a custom extension is needed; whether glass-to-glass is measured through a timestamp
drawn into the frames; how clocks are synchronised across machines and how that error
is reported; what a viewer reports when methods disagree; and what "latency" means in a
report so that two servers can be compared.

---

## Questions

### Q1 · Which single quantity does a report call "latency" when it ranks two servers?

- **Risk if unresolved:** network one-way delay (send stamp to viewer receive) and glass-to-glass (capture to decoded frame) differ by encode, packetisation, jitter-buffer and decode time — tens to hundreds of ms. A report that blends them, or headlines one for server A and the other for server B, makes the comparison meaningless, and the report schema changes twice.
- **Default assumption:** both are reported as separately named metrics, never blended. The server-comparison headline is network one-way delay in ms (p50/p95/p99), because it isolates the server's forwarding; glass-to-glass sits beside it, labelled, and is never used to rank servers.
- **Answer:** default accepted (2026-10-01, Allan Nava, in chat: "accetta le ipotesi").

### Q2 · Must every latency method work against a server that negotiates no header extension on either leg?

- **Risk if unresolved:** MediaMTX v1.21.1 drops abs-capture-time on both legs, and any SFU that answers the SDP drops an extmap URI it does not know. If one supported method depends on a forwarded extension, whipbench reports nothing for such servers, and "no number" gets read as "no latency problem".
- **Default assumption:** yes — at least one method must survive a server that forwards only the RTP payload untouched; a server that strips the extension gets a report that says "network stamp unavailable: extension not negotiated (publisher leg / viewer leg)", never a blank or a zero.
- **Answer:** default accepted (2026-10-01, Allan Nava, in chat: "accetta le ipotesi").

### Q3 · Is WB-1 a decision record only, or does it also implement the chosen methods?

- **Risk if unresolved:** the ticket reads "decide how", but in-frame stamping, clock-offset measurement and the report schema are each several days of work; assuming implementation turns one ticket into five, assuming decision-only leaves 0.0.1's flawed number in every report meanwhile.
- **Default assumption:** WB-1 delivers the decision — method, definitions, report fields and error model — plus follow-up tickets in `BACKLOG.md` for each method; no code changes beyond renaming the 0.0.1 metric so it stops claiming to be "latency".
- **Answer:** default accepted (2026-10-01, Allan Nava, in chat: "accetta le ipotesi").

### Q4 · Does abs-capture-time stay the network stamp, or does whipbench need a custom header extension?

- **Risk if unresolved:** abs-capture-time is defined as the frame's capture time (64-bit NTP, UQ32.32, with an optional estimated-capture-clock-offset field) that mixers and SFUs may legitimately preserve, rewrite or extend — stamping a per-packet send time in it lets a conforming server corrupt the number. A custom extmap URI avoids the clash but is dropped by every server that does not recognise it, which is strictly more servers.
- **Default assumption:** abs-capture-time stays, stamped once per frame in its documented UQ32.32 format from the publisher's wall clock; no custom extension. The report states per leg whether the server forwarded it unchanged, rewrote it, or dropped it.
- **Answer:** default accepted (2026-10-01, Allan Nava, in chat: "accetta le ipotesi").

### Q5 · Is glass-to-glass measured from a timestamp drawn into the frame pixels and read back at the viewer?

- **Risk if unresolved:** a drawn code forces the publisher to encode live (it cannot be drawn into a pre-encoded clip), costs decode-and-read CPU at every viewer that checks it, and is destroyed by heavy quantisation at low bitrate or by a transcoding server. Reading it on every viewer of a large scenario can saturate the load generator and inflate the very delay it measures.
- **Default assumption:** yes — a machine-readable block code (not OCR text) carrying a 64-bit millisecond timestamp, sized to survive the lowest bitrate any scenario uses; read on at most 10 sampled viewers per run, every frame, with the count of unreadable codes reported beside the result.
- **Answer:** default accepted (2026-10-01, Allan Nava, in chat: "accetta le ipotesi").

### Q6 · Where does the in-frame measurement end at a viewer that has no screen — RTP receive, frame complete, decoder output, or simulated playout?

- **Risk if unresolved:** load-test viewers do not render, so "glass" does not exist. A jitter buffer and playout delay (libwebrtc's adaptive target versus none in a bare decoder) add 20-200 ms that belong to the client, not the server, so the endpoint chosen decides whether two servers differ by their own behaviour or by the viewer's buffering.
- **Default assumption:** the endpoint is decoder output — the instant the decoded frame carrying the code exists — with no buffering beyond reassembling a complete frame. whipbench calls this "capture-to-decode" and does not use the word glass-to-glass until a viewer that renders exists.
- **Answer:** default accepted (2026-10-01, Allan Nava, in chat: "accetta le ipotesi").

### Q7 · Must publisher and viewers run on separate machines, and what clock error is acceptable?

- **Risk if unresolved:** one-way delay across two clocks carries their whole offset as error — NTP over the internet ±1-10 ms (worse after a step), chrony on a LAN well under 1 ms — so a 5 ms offset against a 3 ms forwarding delay makes the number noise. Putting everything on one host removes the offset but lets the load generator steal CPU from the server under test.
- **Default assumption:** one host sharing one wall clock is the reference set-up and the only one comparable without caveat; multi-host runs are allowed with an error budget of ±1 ms, and a run whose clock uncertainty exceeds it is marked "not comparable", not rejected.
- **Answer:** default accepted (2026-10-01, Allan Nava, in chat: "accetta le ipotesi").

### Q8 · How is the clock offset between machines measured, and how does it appear in the report?

- **Risk if unresolved:** `chronyc tracking` or `timedatectl` report each daemon's estimate against its own upstream, not the offset between the two whipbench hosts; drift left uncorrected (up to 50 ppm free-running, 180 ms an hour) biases long runs; a figure without an error bar invites a ranking the data does not support.
- **Default assumption:** whipbench measures the offset itself with an NTP-style request/response exchange between its own hosts at the start and end of each run (offset ± RTT/2), interpolates linearly between the two, and prints every cross-host latency as value ± uncertainty in ms, naming the method; it never subtracts an offset it does not report.
- **Answer:** default accepted (2026-10-01, Allan Nava, in chat: "accetta le ipotesi").

### Q9 · What does a viewer report when the network stamp and the in-frame stamp disagree?

- **Risk if unresolved:** capture-to-decode contains the network delay, so it can never be smaller; silently choosing one value hides a server that rewrites abs-capture-time, a misread code or a clock step, and averaging the two yields a number no method measured.
- **Default assumption:** both are reported side by side with their own sample counts and no reconciliation; the run is flagged "methods disagree" when the in-frame p50 falls below the network p50 by more than the combined clock uncertainty from Q8.
- **Answer:** default accepted (2026-10-01, Allan Nava, in chat: "accetta le ipotesi").

### Q10 · What is one latency sample, and which statistics make two reports comparable?

- **Risk if unresolved:** per-packet samples overweight keyframes (dozens of packets, the last queued behind the first); first-packet and last-packet-of-frame timing differ by the frame's serialisation time, several ms for a 1080p keyframe; means hide tails; including ICE/DTLS set-up and the first-keyframe wait skews short runs.
- **Default assumption:** one sample per frame, timed at the arrival of its last packet (RTP marker bit), in ms at 0.1 ms resolution; reports carry count, p50, p95, p99 and max, exclude the first 5 s after first media, and show packet loss and NACK/RTX counts beside them, since retransmitted frames are slow by construction.
- **Answer:** default accepted (2026-10-01, Allan Nava, in chat: "accetta le ipotesi").

---

## Out of scope

Things the ticket might suggest but that we are **not** doing in this task:

- Implementing the methods themselves, if Q3's default holds — each becomes its own backlog ticket.
- Physical glass-to-glass: a camera pointed at a screen, a photodiode, display scan-out and vsync.
- Audio latency and lip-sync (audio/video offset).
- Servers that transcode, and simulcast or SVC layer switching — an in-frame code is not guaranteed to survive re-encoding.
- Hardware time synchronisation (PTP-capable NICs, GPS/PPS); whipbench works with the clock the OS gives it.
- Estimating one-way delay as RTCP round-trip time halved — RTT is per leg and paths are asymmetric.
- Signalling latency and time-to-first-frame (WHIP/WHEP POST to first decoded frame) — a separate metric.

---

## Status

- [x] Questions generated
- [x] Reviewed by a human (2026-10-01, Allan Nava)
- [x] Answers collected (or assumptions explicitly accepted) — all ten defaults accepted, 2026-10-01

> Next phase: **Research**. The ticket is **not** passed to Research — only the
> questions and their answers.
