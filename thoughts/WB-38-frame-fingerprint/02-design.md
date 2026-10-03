# 02 · Design — WB-38 One-way delay by frame fingerprint

**Written against:** `f8e1ff7` · paths from `01-research.md`; pion paths are module-cache paths.

---

## Problem

`run`'s only delay figure, packet transit, needs abs-capture-time on every packet and a shared wall
clock (`internal/report/report.go:110-115`, `:294`). WB-1 made one-way delay by frame fingerprint
the headline (D2, D4), yet nothing reassembles a frame, logs its send, or spots one a loop late.

**Ticket:** WB-38 · https://github.com/Allan-Nava/whipbench/blob/main/BACKLOG.md

---

## Proposed solution

Each clip frame gets a 64-bit fingerprint at load (D1). In `run` the publisher logs t0 per
absolute frame index k just before the frame's first `WriteRTP` (D2). Each viewer reassembles with
its own small reassembler that records every packet's arrival (D4); a complete frame is hashed and
matched to the latest send of its clip index before t1, the arrival of its last (marker) packet
(D5). The RTP timestamp interval since the viewer's last good match is Q1's "evidence of its own":
when it proves a match a whole loop off, the sample is invalid (D3). Retransmissions are not counted
here (D6). The figure is a `oneWayDelay` source block, per viewer and pooled (D7); WB-1's two name
guards change what they protect (D8). `view` reports it unavailable.

### Diagram

```
 clip load ─ fingerprint(frame i), i < N ─► table: fp → i, duplicates   (D1)
 publisher  frame k (i = k mod N): t0 = Now(); log[k] = t0; WriteRTP pkt 1 … pkt n (M=1)   (D2)
                                                   │ server │
 viewer     ReadRTP → (pkt, arrival) → reassembler keyed by RTP ts                         (D4)
            complete? ─ no, 1 s after its first packet ─► incompleteFrames
              └ yes ─ t1 = arrival of last packet (marker) (D5) ─ depacketise ─ fingerprint
                 ├ not in table ─► unmatchedFrames      ├ duplicate ─► not sampled
                 └ kL = latest k ≡ i (mod N), log[k] ≤ t1 ─ RTP Δ says kL is m·N too new? (D3)
                      ├ yes ─► invalid                 └ no ─► sample t1 − log[kL] (monotonic)
```

### Components to touch

| Component | Path | Kind of change |
|---|---|---|
| Fingerprint, table, send log, match | new package (name for Structure) | new: pure, tested on synthetic input |
| Reassembler | new package (name for Structure) | new: packets + arrival → complete frames |
| Publisher | `internal/publisher/publisher.go:33-45`, `:244-269` | modify: `Config` takes the send log; t0 logged before `:257` |
| Viewer | `internal/viewer/viewer.go:54-68`, `:302-340`, `:347` | modify: feed the reassembler at `:312`; per-viewer block |
| Runner | `internal/runner/runner.go:66`, `:82`, `:132` | modify: one table + log, shared like `*metrics.Live` |
| Report, Markdown, stdout | `internal/report/report.go:110-115`, `:173`, `:289`; `markdown.go:46-72`; `cmd/whipbench/main.go:221`, `:304-311` | modify: block, pooling, Method line, row, headline |
| Test relay | `internal/testserver/testserver.go:31-41`, `:196-225` | modify: an option that clears the marker (Q9 test) |
| Guards, docs | `internal/report/report_test.go:104-119`; `internal/metrics/metrics_test.go:41-42`; README table; CHANGELOG | modify (D8); definition in three places (`CLAUDE.md:44-45`) |

---

## Decisions

### D1 · Fingerprint: first 64 bits of SHA-256, computed once per clip frame at load

- **Choice:** VP8 hashes the whole frame; H.264 hashes its VCL NAL units (types 1-5) in order, each
  with its header byte, no start codes — `SplitAnnexB` (`internal/clip/clip.go:200`) on both sides.
  The table is built from `clip.Frames` at load, for any clip; a fingerprint seen twice in one loop
  goes to the duplicate set (Q7), sent but never sampled, its size reported. No extra check of a
  `--clip` file: the same table applies to it (`cmd/whipbench/main.go:150-159`).
- **Why:** `Frame.Data` is Annex-B with start codes, SPS, PPS, SEI (`clip.go:178-193`), and the wire
  is not that byte for byte (`rtp@v1.10.5/codecs/h264_packet.go:106-141`); VCL bytes matched 450/450
  (`evals/2026-10-02-mediamtx-fingerprint.md:17-26`), with SHA-256. No new dependency (`CLAUDE.md:58-59`).
- **Rejected:** FNV-1a 64 — no measured gain, not the eval's hash; `hash/maphash` — seeded per
  process, differs between runs; hashing `Frame.Data` — fails on every H.264 frame (Q8 contradiction).
- **Cost:** VCL = types 1-5 is unverified against Table 7-1 (research blind spot). **Reversible?** yes.

### D2 · Send log keyed by absolute frame index k, not by fingerprint (contradiction 4, the key)

- **Choice:** a ring of t0 per k over the last 4 loops (4N entries), one writer, many readers. The
  publisher takes t0, writes `log[k]`, **then** calls the first `WriteRTP` (`publisher.go:257`), so no
  viewer can hold a frame whose send is not yet logged. `publish` passes no log: a no-op.
- **Why:** every k is sent once, in order; a 500 ms slip resets only the schedule origin, no
  catch-up (`publisher.go:229-233`), so it changes the spacing of t0s and nothing else. t0 is the
  actual write (WB-1 D2). "Latest send of that fingerprint" = latest k ≡ i (mod N): the ticket's rule.
- **Rejected:** a map fingerprint → list of t0 (the ticket's wording) — loses k, which D3 needs;
  keying by RTP timestamp — random base per run (`publisher.go:211-212`), re-based by servers (WB-1
  D4); a per-frame callback out of `Stream` (`publisher.go:202`) — a hook where a value will do.
- **Cost:** a lock or atomics on a hot path read per frame per viewer. **Reversible?** yes.

### D3 · Loop length in frames; aliasing caught from the RTP timestamp interval (contradiction 4, Q1)

- **Choice:** the loop is N = `len(clip.Frames)` frames, never `Clip.Duration()`. Each viewer keeps
  an anchor (kA, tsA) from its last valid match. For a new frame, Δ = int32(ts − tsA) / `Ticks`. If
  Δ is whole and kL − (kA + Δ) = m·N with m ≥ 1, the match is m loops too new: **invalid**, anchor
  unchanged. Otherwise the sample counts and the anchor moves to (kL, ts) — no evidence, kept (Q1).
  The block reports `loopFrames` (N) and `loopMinMs`, the shortest t0[k+N] − t0[k] the log saw: the
  delay above which the method is blind without the RTP check.
- **Why:** H.264's 30 fps is hard-coded (`clip.go:231`), so `Duration()` is a guess for H.264 and
  slips stretch a loop in time (`publisher.go:229-233`); N frames is exact. Intervals survive where
  bases do not (WB-1 D4), and `Timestamp(base, k)` = base + k·`Ticks` (`clip.go:63-65`) makes Δ an
  exact frame count whatever the real frame rate. The frame itself is the evidence.
- **Rejected:** loop = `Duration()` — wrong for any non-30 fps Annex-B clip; recovering the true
  delay from Δ instead of marking it invalid — puts the server's timestamps under the headline,
  which WB-1 D4 refused; a viewer-side stall detector (gap > one loop ⇒ invalid) — cannot tell a
  4 s outage followed by an on-time frame from a frame 4 s late.
- **Cost:** **weak spot:** a viewer's *first* match is taken at face value, so a stream already a
  loop late at join aliases throughout; a server that re-times RTP timestamps turns the check off
  silently (no evidence ⇒ kept). **Departs from Q1's wording** ("the loaded clip's duration"): the
  loop is a frame count and its time span is measured. **Reversible?** yes.

### D4 · A small reassembler of our own, arrival recorded per packet (contradiction 2)

- **Choice:** per viewer, packets grouped by RTP timestamp, each with its arrival (`viewer.go:312`),
  first copy of a sequence number wins. A frame is **complete** when its last packet is known (D5)
  and every seq from its first to its last arrived, and seq first−1 arrived with an earlier
  timestamp — so a lost head is never hashed. It is **incomplete** 1 s after its first packet
  arrived, a constant printed in the Method line. Only complete frames are depacketised, with a
  fresh pion `VP8Packet` / `H264Packet` per frame (`rtp@v1.10.5/codecs`). A frame those reject
  (STAP-B, MTAP, FU-B, `h264_packet.go:313`) is a server re-packetisation: `unmatchedFrames` (Q5).
  Frames pending when the viewer stops are counted nowhere, and the Method line says so.
- **Why:** samplebuilder pops a frame only when the next packet arrives, records no arrival, gives
  up by seq count or an optional time bound (`webrtc/v4@v4.2.22/pkg/media/samplebuilder/`
  `samplebuilder.go:155-179`, `:249-254`, `:311-318`, `:389-394`); `H264Packet` keeps stale FU bytes
  after a lost end fragment (`h264_packet.go:297`) — a corrupt hash that blames the server.
- **Rejected:** wrap samplebuilder, arrivals per timestamp beside it — two buffers in step, a
  give-up that is no fixed window, a depacketiser stateful across frames; extend `keyframes.add`
  (`rtpstats.go:233`, WB-1 D2) — keyframes only, headers only; a window in sequence numbers — its
  time length moves with bitrate. **Departs from the ticket**, which names samplebuilder: sign-off.
- **Cost:** new code on the read path under `pr.mu` (`viewer.go:303-340`). **Reversible?** yes.

### D5 · t1 = arrival of the frame's last packet; the marker only confirms it early (Q4, Q9)

- **Choice:** last = highest seq of the timestamp. With a marker it is the marker packet, original
  or retransmitted, and the frame can close at once. Without one, the frame closes when a later
  timestamp's packet bracketing it arrives, and t1 is still the last packet's own arrival. A stream
  switches to the `timestamp` rule on its first complete frame with no marker and never switches
  back; each viewer reports `frameEnd: marker | timestamp`.
- **Why:** the publisher sets M on the last payload (`publisher.go:249`), so both rules name the same
  packet whenever a marker exists — one definition, not two (Q9's risk). MediaMTX keeps the marker
  (`evals/2026-10-02-mediamtx-fingerprint.md:31-34`); no test produces a markerless stream
  (`testserver.go:196-225`), hence the relay option.
- **Rejected:** samplebuilder's pop or the next timestamp's first packet — up to a frame late (Q9,
  `samplebuilder.go:249-254`); the frame's latest arrival — Q4 chose the marker; D6 keeps it for WB-41.
- **Cost:** a gap filled after the marker is invisible in the figure (Q4, accepted). **Reversible?** yes.

### D6 · Retransmissions: none counted here; completion time kept for WB-41 (contradiction 1)

- **Choice:** no retransmission or late-completion count in the report; NACK-recovered packets are
  sampled like any other (ticket). The reassembler records each frame's completion instant, unused
  here, so WB-41's `lateCompletedFrames` is a read, not a rewrite.
- **Why:** RTX is not registered (`internal/rtc/rtc.go:31-34`), `ReadRTP` attributes are dropped
  (`viewer.go:308`), there is no reorder counter: a retransmission and a reordered packet look the
  same. `README.md:73` already gives "its sample window and retransmissions" to WB-41.
- **Rejected:** `lateCompletedFrames` here — WB-41's key, taken early; "retransmitted frames" — no
  signal, it counts reorders too; negotiate RTX — changes the media path under test (WB-1 D7).
- **Cost:** **departs from Q4's default** ("frames that needed a retransmission are counted beside
  the figure"): until WB-41, the figure ships without that count. Needs sign-off. **Reversible?** yes.

### D7 · Report: `oneWayDelay` is a list of source blocks, per viewer and pooled

- **Choice:** `oneWayDelay: [{source: "fingerprint", available, reason, frameEnd, completeFrames,
  incompleteFrames, samples, invalid, unmatchedFrames, ms}]` per viewer; the pooled block adds
  `viewers`, `viewersByFrameEnd`, `duplicateFrames`, `loopFrames`, `loopMinMs`. Histograms merge
  (`internal/stats/stats.go:111`), reasons follow `mostCommon` and "pooled over X of Y"
  (`report.go:220-232`, `:256`). Zero samples ⇒ unavailable with the counts in the reason (Q5);
  `view` ⇒ unavailable, "the send log lives in the `run` process" (Q2). Schema stays v0: a key is
  added, none changes meaning (`report.go:31-32`, `CHANGELOG.md:9-15`). It heads the Markdown table
  and the stdout line; packet transit stays, unchanged, below it (Q6).
- **Why:** WB-40 gives each source one block with a `source` field, and WB-39 adds `stamp` under the
  same key; a list grows without changing the key's type.
- **Rejected:** one object — WB-39 would change its type, a schema bump, and v1 is WB-33's (WB-1
  D1); a map keyed by source — source both key and field; WB-40's `comparable`, `uncertaintyMs`.
- **Cost:** `jq` needs `select(.source==…)`; WB-40's list says `frames`/`unmatched`, the ticket's
  `unmatchedFrames` wins and WB-40 adopts it. **Reversible?** until a report ranks by it (WB-5).

### D8 · The two name guards change what they guard (contradiction 3)

- **Choice:** `report_test.go:118-119` becomes: "one-way delay" occurs in a report's JSON exactly
  once, in the fingerprint Method line, never in packet transit's Method line or block; `latency`
  stays forbidden (`:115`). `metrics_test.go:41-42` keeps its assertion — no `one_way_delay` series
  — with a message that names the condition: a source label. WB-38 adds no Prometheus series.
- **Why:** D1 rejected one name with two meanings, a per-packet stamp called one-way delay. Now
  the name has one meaning, per frame, first-packet send to last-packet arrival; `source` says how
  t0 was obtained, not what is measured (WB-39's stamp is the same t0). An unlabelled series would
  mix both sources once WB-39 lands — D1's "series that changes meaning silently" — so it stays shut.
- **Rejected:** delete both guards — nothing stops packet transit taking the name back; an
  unlabelled series now — D1's failure in waiting; a labelled one — outside the ticket.
- **Cost:** no live view of the headline during a run; dashboards wait. **Reversible?** yes.

---

## Impact

| Area | Impact | Mitigation |
|---|---|---|
| Report JSON, compat | new `oneWayDelay` list per viewer and pooled; v0 kept; `packetTransit` untouched | no key changes meaning; old reports not rewritten (Q6) |
| Prometheus | unchanged | D8; follow-up below |
| CPU, memory | reassembly + SHA-256 per frame on every viewer, always on (Q10); ~1 s of payload per viewer | unmeasured, the Method line says so (WB-25); bounded by D4's window and D2's ring |
| Security | none: no port, no input beyond the RTP already read | — |

---

## What we are NOT doing

- WB-39's stamp, WB-40's clock, topology and comparability, WB-41's window, NACKs and late frames.
- Split runs: `view` reports the figure unavailable (Q2); no cross-host send log.
- Fixing the 30 fps hard-code in `clip.Load` (`clip.go:231`); changing the clip loop.
- A keyframe/delta split (Q3); a minimum match fraction (Q5); "Out of scope" in `00-questions.md`.

---

## More research needed

Facts the design assumes but `01-research.md` did not verify:

- [x] D2: a schedule slip never skips or repeats a frame index k — verified 2026-10-03 by reading
  `internal/publisher/publisher.go:227-233`: `k` advances by one per iteration and a slip moves only
  `origin`; the one edge is a frame cut short when the connection closes, and its k is never reused.
- [ ] D1: H.264 VCL NAL types are exactly 1-5 (Table 7-1 was unreachable; types from pion comments).
- [x] D4: a fresh `H264Packet` on one complete frame, then `SplitAnnexB`, gives the VCL bytes
  `SplitAnnexB` gives on `Frame.Data` — verified 2026-10-03 offline: the embedded H.264 clip through one
  long-lived `H264Payloader` (MTU 1200, as the publisher) and a fresh depacketiser per frame, two
  loops: 240/240 frames' VCL equal, 8 STAP-A packets harmless; VP8 120/120 frames equal. The clip's NAL
  types are 1, 5, 6, 7, 8 — its VCL units (1, 5) sit inside 1-5. Implement should keep this as a test.
- [ ] D4: whether 1 s outlasts pion's NACK retries and responder buffer, and whether `DropEvery`
  yields retransmissions or only holes (research blind spot) — the lossy test depends on it.
- [ ] D3: whether RTP timestamp intervals survive servers other than MediaMTX (WB-4).

> If this list is not empty, consider a short targeted Research round **before**
> moving to Structure. It costs less than the rework.

### Proposed follow-ups (titles only)

- Live one-way delay series with a source label
- Frame rate for Annex-B clips from the clip, not a constant
- One-way delay split by keyframe and delta frame

---

## Review

| Comment | From | Status | Resolution |
|---|---|---|---|
| D4 replaces the samplebuilder the ticket names; D6 moves Q4's retransmission count to WB-41; D3 measures the loop in frames, not Q1's clip duration | Allan Nava, 2026-10-03 | resolved | all three accepted as written |
| D2 and D4 rest on two unverified facts: a slip never skips a frame index; a fresh depacketiser recovers the VCL bytes | review, 2026-10-03 | resolved | both verified 2026-10-03 (#61): publisher.go:227-233; 240/240 H.264 and 120/120 VP8 frames |

---

## Status

- [x] Design written
- [x] Anchored to research facts (every claim has a path)
- [x] Alternatives documented
- [x] Reviewed by the team (2026-10-03, Allan Nava, summary)
- [x] Comments resolved
- [x] Approved (2026-10-03, Allan Nava, in chat: "ok approvo")

> Next phase: **Structure**. It receives: this file only.
