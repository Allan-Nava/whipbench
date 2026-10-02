# 02 · Design — WB-1 Latency method

**Written against:** `1cc117d`

---

## Problem

0.0.1's only delay figure is per-packet send stamp to arrival, carried in abs-capture-time, so it
vanishes whenever a server drops the extension — MediaMTX v1.21.1 does on both legs
(`evals/2026-10-01-mediamtx-local.md:44-48`). No report has ever carried a delay for a real
server; whipbench needs one defined, comparable "how late is the video" number.

**Ticket:** WB-1 · https://github.com/Allan-Nava/whipbench/blob/main/BACKLOG.md

---

## Proposed solution

One quantity ranks servers: **one-way delay**, per frame, from the publisher writing the frame's
first packet to the viewer receiving its last (marker) packet; no metric is called "latency". The
0.0.1 figure is renamed **packet transit** — WB-1's only code change — and retired when D2 ships
(D1). The send instant has two **sources**, side by side: the **stamp**, abs-capture-time once per
frame (D3), and the **fingerprint**, new here — the viewer hashes each reassembled frame and looks
up when the in-process publisher sent it, with no extension and no decoder (D4); it meets Q2 and
is the headline source in `run`. **Capture-to-decode** (Q5/Q6) is decided but deferred (D5).
Every figure carries topology, clock method, uncertainty and a `comparable` verdict (D6-D8).

### Diagram

```
 publisher (run: one process)               server          viewer
 frame k ─┬─ t0 = Now() before 1st WriteRTP ──► SFU ──► pkt 1 … pkt n (marker) → t1 = arrival
          ├─ abs-capture-time(t0) on pkt 1 only   (D3: may be dropped or rewritten)
          └─ sendlog[hash(frame k)] += t0, monotonic  (D4)   reassemble → hash → sendlog → t0
 one-way delay     = t1 − t0   source: stamp (wall clock) | fingerprint (monotonic)
 capture-to-decode = t2 − t0   t2 = keyframe decoded, index read from pixels (D5, deferred)
 packet transit    = per-packet arrival − per-packet send stamp  (0.0.1, renamed by WB-1)
```

### Components to touch — WB-1 changes only the rename (Q3); the rest is follow-up items

| Component | Path (from `01-research.md`) | Kind of change |
|---|---|---|
| README, CI phrase check | `README.md:53`, `:62-71`; `scripts/check-repo.sh:15-18` | modify: "packet transit", "packet transit: unavailable"; "not glass-to-glass" stays |
| Method string, JSON keys | `internal/report/report.go:104`, `:290`; `internal/viewer/viewer.go:55-68`, `:92` | modify: `latency` → `packetTransit`, type `Latency` → `PacketTransit` |
| Markdown, Prometheus, stdout | `internal/report/markdown.go:58`, `:77`; `internal/metrics/metrics.go:19`, `:93-94`; `cmd/whipbench/main.go:272` | modify: `whipbench_packet_transit_seconds` |
| Tests on the strings; CHANGELOG | `internal/report/report_test.go:62`, `:98`; `internal/runner/runner_test.go:143`, `:159`; `internal/metrics/metrics_test.go:9`; `CHANGELOG.md:3-4` | modify; one `[Unreleased]` line citing WB-1 |

---

## Decisions

### D1 · Names: 0.0.1 becomes "packet transit", "one-way delay" goes to the headline (contradiction 1)

- **Choice:** rename the 0.0.1 metric wherever it is named (table above); no metric is called
  "latency"; "one-way delay" goes to D2 when it ships, under new keys (`oneWayDelay`). Schema stays
  `whipbench.report/v0` (`report.go:30-31`): no key changes meaning — one goes, one comes.
- **Why:** 0.0.1 calls a per-packet send stamp "one-way delay" (`README.md:53`, `markdown.go:58`,
  `metrics.go:19`, `main.go:272`); Q1/Q4/Q10 define a per-frame quantity. Q3 allows this rename.
- **Rejected:** keep the name on 0.0.1, rename the headline — Q1's name would sit on the wrong
  number; redefine the name in place when D2 ships — one name, two meanings, a Prometheus series
  that changes meaning silently; dual-emit both series — keeps the wrong name alive; bump the
  schema — the next obvious string, v1, belongs to WB-33.
- **Cost:** 0.0.1 dashboards break; a parser of `latency` meets a missing key, not an error
  (**weak spot**); `scripts/check-repo.sh:16` changes in the same commit or CI fails. **Reversible?**
  yes, until D2 ships under the freed name.

### D2 · One-way delay: one sample per complete frame

- **Choice:** t0 = publisher `time.Now()` just before the frame's first `WriteRTP`; t1 = viewer
  arrival of its marker packet, sampled only if every packet arrived (NACK-recovered included);
  incomplete frames are counted, not sampled. Prose: "network and server forwarding", never "glass-to-glass".
- **Why:** Q10. The actual write, not the due time (`publisher.go:213-243`), keeps slips (`:229-233`)
  off the server's bill; marker on the last packet (`:248-252`); completeness extends `rtpstats.go:233-254`.
- **Rejected:** t0 = due time — bills the load generator's scheduling to the server; t1 = first
  packet — Q10, hides how a server paces big frames; per-packet samples — overweight keyframes.
- **Cost:** includes the frame's serialisation (several ms for a big keyframe), equal for every
  server. **Weak spot:** a server dropping the marker bit (MediaMTX unverified) needs a fallback
  end, the last packet before the next RTP timestamp. **Reversible?** no once reports rank by it.

### D3 · Stamp source: abs-capture-time once per frame, classified per leg (contradiction 4)

- **Choice:** abs-capture-time = t0 on the frame's **first packet only**, joined by RTP timestamp.
  In `run` the publisher keeps the values sent; one outside that set is **rewritten**, never
  sampled. Per leg: publisher `negotiated | dropped` (`publisher.go:157-160`); viewer `forwarded |
  rewritten | dropped | unverified` (split). Replaces the per-packet stamp (`publisher.go:253-257`).
- **Why:** Q4, once per frame. The spec's SHOULD NOT (every packet) is met; its "every second" is
  a suggestion, deliberately not followed. LiveKit rewrites by default (PR #4812) and the spec
  allows it, so rewriting is an outcome to label, not an error.
- **Rejected:** spec cadence (~1/s) extrapolated by RTP timestamp — the RTP ts is the clip's
  nominal schedule (`clip.go:63-65`), so slips (`publisher.go:229-233`) return as delay; the stamp
  on every packet — more SHOULD-NOT for robustness NACK gives, if retransmission keeps extensions
  (**More research**); a custom URI — Q4, dropped by more servers; detecting rewrites by the
  offset field (`pion/rtp@v1.10.5/abscapturetimeextension.go:124`) — a rewriter may omit it.
- **Cost:** rewrite detection only in `run`; the stamp stays a wall-clock difference
  (`viewer.go:328`), exposed to clock steps (D6). **Reversible?** yes.

### D4 · Fingerprint source: hash of the frame against the publisher's send log (new; meets Q2)

- **Choice:** in `run` the publisher logs t0 per frame under a 64-bit hash of its depacketised
  bytes (VP8 frame; H.264 VCL NAL units only); the viewer reassembles (pion `samplebuilder`,
  `codecs.VP8Packet`/`H264Packet`), hashes, and takes the latest send of that hash before t1.
  Monotonic clock both ends. The **headline source** when available; the stamp sits beside it.
- **Why:** Q2 needs a method that survives a payload-only server; MediaMTX is one
  (`evals/2026-10-01-mediamtx-local.md:44-48`). Clips go out byte for byte (`clip.go:1-8`), so the
  publisher knows each frame's bytes; the relay leaves payload alone (`testserver.go:196-225`).
- **Rejected:** the drawn code as the Q2 method (Q5's role) — needs an inter-frame decoder pure Go
  lacks (`golang.org/x/image@v0.46.0/vp8/decode.go:343-346`), adds nothing while the payload is
  untouched; matching by RTP timestamp — servers re-base it, only intervals are known to survive
  (`evals/2026-10-01-mediamtx-local.md:39`); per-packet hashes — break on re-packetisation.
- **Cost:** `run` only (the log is in memory); ambiguous past the 4 s clip loop
  (`scripts/make-clips.sh:7-33`), such samples invalid; byte-identical frames excluded at load; a
  bitstream rewrite (SPS/PPS injection, transcoding) gives `unmatchedFrames`; per-viewer CPU
  unmeasured. **Reversible?** yes. **Departs from an accepted default** (Q5's role): needs sign-off.

### D5 · Capture-to-decode: frame index baked into the clips, keyframes only, deferred (contradiction 3)

- **Choice:** `scripts/make-clips.sh` draws a block code (16-bit frame index plus check bits, sized
  for 600 kbit/s) into each source frame; time comes from D4's send log keyed by index. Viewers
  decode **keyframes only** with `golang.org/x/image/vp8` (BSD-3, pure Go), VP8 only, ≤ 10 sampled
  viewers (Q5), endpoint decoder output (Q6), unreadable codes counted. Built in v0.6.0 beside
  transcoding (WB-31), where it first measures something D4 cannot.
- **Why:** nothing can be drawn at run time — pre-encoded clips, no encoder (`clip.go:1-8`);
  `x/image/vp8` is the one vetted pure-Go decoder, keyframes only (`decode.go:343-346`); rule 6
  (`CLAUDE.md:41-57`) is enforced by `CGO_ENABLED=0` builds (`ci.yml:19-39`); GOP 30 gives one
  sample per second per viewer (`make-clips.sh:7-33`).
- **Rejected:** cgo libvpx/openh264 — breaks rule 6 and four CI targets; `gen2brain/vpx` and the
  other pure-Go decoders — pre-v1 or self-reported, arm64 build unverified, each needs a CLAUDE.md
  reason (`CLAUDE.md:118-120`); a 64-bit timestamp drawn live (Q5 as written) — needs a live
  encoder, none researched; ship in v0.1.0 — against a non-transcoding server it is D4 plus
  whipbench's own decode time, a client property.
- **Cost:** keyframes are the largest frames — biased high, labelled so; no H.264; clips regenerated
  once. Drops Q5's in-frame timestamp and "every frame" — **departs from a default**. **Reversible?**
  yes; a vetted inter-frame decoder upgrades it to every frame.

### D6 · Topology, clocks and hosts in the report (contradiction 5)

- **Choice:** every report gains `topology` (`single-process` | `split`) and `clock{method,
  offsetMs, uncertaintyMs, stepDetected}`, no host names. `run`: method `monotonic` (fingerprint)
  or `same-wall-clock` (stamp) with a step check — wall and monotonic elapsed time > 0.1 ms apart
  at run end sets `stepDetected`. `view`: `split`; with `--clock-peer` it runs Q8's exchange
  against a responder in `publish` — start, end and every 30 s, offset ± min-RTT/2,
  piecewise-linear — else method `none`, `comparable:false`, "publisher clock not measured".
- **Why:** split runs already work (`main.go:71-87`, `:142-208`) and look single-host (`report.go:58-69`).
  `Sub` is monotonic only when both operands are (`$GOROOT/src/time/time.go:1227-1232`).
- **Rejected:** RTCP SR NTP/RTP pairs — SFUs originate their own SRs (RFC 8079 §3.2), pion keeps
  only LSR/DLSR (`interceptor@v0.1.49/pkg/report/receiver_stream.go:113-118`); `chronyc` — Q8,
  each daemon's view of its upstream; start and end only (Q8 as written) — kept, plus mid-run
  points for non-linear slewing; host names — new private detail, a report names only the server's
  hosts (`report.go:58-69`, `CLAUDE.md:51-53`); wait for WB-24 — split runs exist now.
- **Cost:** a second port between load hosts. **Reversible?** yes, but field names are schema.

### D7 · Sample window, retransmission and resolution (contradiction 2)

- **Choice:** scenario key `excludeFirstSeconds` (default 5), per viewer from its first RTP packet.
  No RTX counts; beside one-way delay go NACKs sent (pion stats interceptor, registered
  `rtc.go:108-109`, unread) and `lateCompletedFrames` (a gap filled after the marker). Keep the 1 %
  histogram (`stats.go:55-147`); the Method line says "±0.5 % of value", values print to 0.1 ms.
- **Why:** `warmupSeconds` is the pause before the first viewer (`scenario.go:43-45`); a per-run
  window excludes ramped viewers unevenly. RTX is not negotiated (`rtc.go:31-34`). ±0.5 % is
  ±1 ms at 200 ms, Q7's multi-host budget; below 20 ms it already resolves 0.1 ms.
- **Rejected:** reuse `warmupSeconds` — changes a key in use; negotiate RTX — changes the media path
  under test; growth 1.0005 (~20× the buckets) or exact samples — no measured need.
- **Cost:** short runs lose 5 s per viewer; Q10's "0.1 ms" becomes display precision. **Reversible?** yes.

### D8 · Error model and comparability

- **Choice:** one block per source: `{source, available, reason, frames, samples, invalid,
  rewritten, unmatched, ms{n,min,p50,p95,p99,max}, uncertaintyMs, comparable, notComparableReason}`;
  unavailable never carries a number (`viewer.go:342-358`, `report.go:109-114`). Two reports rank
  only if both are comparable, share scenario, clip and source, and uncertainty ≤ 1 ms (Q7).
  Flags: `sourcesDisagree` (stamp `forwarded` yet its p50 differs from the fingerprint's beyond
  uncertainty); Q9's `methodsDisagree` once D5 ships. No averaging, no reconciliation (Q9).
- **Why:** Q1, Q9 and the never-a-fake-number rule (`CLAUDE.md:45-47`).
- **Rejected:** a merged "best available source" — A on fingerprint, B on stamp is the blend Q1
  forbids; ranking on p50 alone — tails are what viewers notice.
- **Cost:** some report pairs refuse a ranking — the point. **Reversible?** yes, until WB-33's schema.

---

## Impact

| Area | Impact | Mitigation |
|---|---|---|
| Report JSON, scenario | WB-1: `latency` → `packetTransit`; later `oneWayDelay`, `topology`, `clock`, `excludeFirstSeconds` | no reused key; v0 until WB-33 (D1); `warmupSeconds` untouched |
| Prometheus, backward compat | series renamed; 0.0.1 dashboards and parsers break (D1) | intended by Q3; CHANGELOG line, not shimmed |
| Performance | reassembly + hash on every viewer (D4); keyframe decode on ≤ 10 (D5) | unmeasured — **More research**; WB-25's ceiling |
| Security / privacy | clock responder opens a port (D6) | opt-in flag; no host names in reports |

---

## What we are NOT doing

- Implementing any method in WB-1 (Q3); a live encoder or a timestamp drawn at run time (D5).
- The fingerprint and rewrite detection in split runs — they need a cross-host send-log join.
- Everything under "Out of scope" in `00-questions.md`.

---

## More research needed

Facts the design assumes but `01-research.md` did not verify:

- [ ] D4: the WB-4 servers forward depacketised VP8 frames / H.264 VCL NALs byte for byte — MediaMTX v1.21.1 does (451/451, 450/450; `evals/2026-10-02-mediamtx-fingerprint.md`); the others are unverified.
- [x] D4: the 120 encoded frames of each clip are pairwise byte-distinct — 120/120 for both clips (2026-10-02, `evals/2026-10-02-mediamtx-fingerprint.md`).
- [x] D2: MediaMTX preserves the marker bit — one per frame on both codecs (2026-10-02, `evals/2026-10-02-mediamtx-fingerprint.md`).
- [ ] D3: pion's NACK responder resends the stored packet with its header extensions.
- [ ] D7: the viewer's stats-interceptor NACK count means "NACKs sent"; recovered packets vs `tooLate` (`internal/rtpstats/rtpstats.go:155-162`).
- [ ] D4/D5: CPU per viewer of reassembly + hash at scale, and of one `x/image/vp8` keyframe decode at 640×360.
- [ ] D5: a code drawn by `make-clips.sh` survives libvpx at 600 kbit/s and reads back after decode.

> If this list is not empty, consider a short targeted Research round **before**
> moving to Structure. It costs less than the rework.

---

## Proposed follow-up items

For `BACKLOG.md`, no ids assigned; a later phase writes them.

- **WB-n — One-way delay by frame fingerprint**: D2 + D4 in `run`, every viewer; WB-5's headline.
- **WB-n — abs-capture-time once per frame**: D3's per-leg classification; retires packet transit.
- **WB-n — Topology, clock and comparability in the report**: D6 fields, step check, D8 blocks and ranking.
- **WB-n — Sample window and retransmission beside delay**: D7's window, NACKs, late frames, Method line.
- **WB-3, rewritten** — the clock exchange: `publish` responder, `view --clock-peer` (D6).
- **WB-2, rewritten and moved to v0.6.0** — baked index, keyframe capture-to-decode, VP8 (D5).
- **WB-4, widened** — per server, also payload byte-identity and the marker bit (D4, D2).

---

## Review

| Comment | From | Status | Resolution |
|---|---|---|---|
| | | open / resolved | |

---

## Status

- [x] Design written
- [x] Anchored to research facts (every claim has a path)
- [x] Alternatives documented
- [ ] Reviewed by the team (<date>, <who>, <how: read in full / summary / automated review>)
- [ ] Comments resolved
- [ ] Approved (<date>, <who>, <how: in writing / in chat / in review>)

> Next phase: **Structure**. It receives: this file only.
