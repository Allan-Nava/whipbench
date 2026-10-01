# 00 · Questions — WB-1 Latency method

**Written against:** `<commit — git rev-parse --short HEAD when this phase ran>`

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

### Q1 · <question in one line>

- **Risk if unresolved:** <what goes wrong>
- **Default assumption:** <what we assume in order to proceed>
- **Answer:** _(to be filled — human)_

### Q2 · <...>

- **Risk if unresolved:**
- **Default assumption:**
- **Answer:** _(to be filled — human)_

---

## Out of scope

Things the ticket might suggest but that we are **not** doing in this task:

- <...>

---

## Status

- [ ] Questions generated
- [ ] Reviewed by a human (<date>, <who>)
- [ ] Answers collected (or assumptions explicitly accepted)

> Next phase: **Research**. The ticket is **not** passed to Research — only the
> questions and their answers.
