# 03 · Structure — WB-38 One-way delay by frame fingerprint

**Written against:** `5c15cca`

---

## Reference

Design: [`02-design.md`](./02-design.md) — D1 to D8, approved 2026-10-03, with three accepted
departures: D4 (own reassembler, not samplebuilder), D6 (retransmissions left to WB-41), D3 (the
loop measured in frames).

The two new packages the Design leaves to Structure are named here:

- `internal/fingerprint` — fingerprint, clip-side table, send log, match and loop aliasing (D1, D2, D3);
- `internal/reassembler` — packets plus arrival in, complete frames out (D4, D5, D6).

**Gate shared by every step** (CI runs the same three, `.github/workflows/ci.yml:28-29`); a step is
done only when its own command below **and** this gate exit 0:

```
test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./...
```

Test cases are named by what they prove; their code is Plan's and Implement's.

---

## Steps

### S1 · Fingerprint and the clip-side table

- **Goal:** a 64-bit fingerprint per frame and the table fp → clip index i, with its duplicate set (D1).
- **Touches:** `internal/fingerprint/fingerprint.go` (with the package comment that carries the
  definition, `CLAUDE.md:44-45`), `internal/fingerprint/fingerprint_test.go`
- **Depends on:** — (none)
- **Who:** agent
- **Proves:**
  - a VP8 frame hashes over its whole payload;
  - an H.264 frame hashes over its VCL NAL units only, header byte kept, start codes, SPS, PPS and
    SEI ignored, so `Frame.Data` and the same VCL units without the parameter sets give one value;
  - the value is the first 64 bits of SHA-256: a fixed vector, equal across processes (the
    `maphash` rejection);
  - both embedded clips build a table of N = `len(clip.Frames)` entries with no duplicates;
  - a synthetic clip with one frame repeated puts that fingerprint in the duplicate set and out of
    the table's sampled entries, and reports the set's size.
- **Open (carried from More research, D1):** VCL = NAL types 1-5 is unverified against H.264
  Table 7-1. S1 implements 1-5 behind one named predicate so H1's answer changes one line.
- **Verify (done when):** `go test -race -count=1 ./internal/fingerprint/ -run 'TestFingerprint|TestTable'` exits 0
- **Parallel:** yes, with S2, S3, S5, H1 — S2 shares the package directory but not a file
- **Repo state after:** working; a package nothing imports yet

### S2 · Send log and match with loop aliasing

- **Goal:** the ring of t0 per absolute frame index k, the "latest k ≡ i (mod N) sent before t1"
  lookup, and the per-viewer anchor that marks a match m loops too new as invalid (D2, D3).
- **Touches:** `internal/fingerprint/sendlog.go`, `internal/fingerprint/sendlog_test.go`,
  `internal/fingerprint/match.go`, `internal/fingerprint/match_test.go`
- **Depends on:** — (none: it takes the clip index i, not a fingerprint)
- **Who:** agent
- **Proves:**
  - the ring holds 4N entries; an older k is overwritten, never returned;
  - the lookup returns the latest kL ≡ i (mod N) with log[kL] ≤ t1, and never a k whose t0 is
    after t1 (a send not yet logged);
  - one writer and many readers run clean under `-race`;
  - a schedule slip widens the spacing of t0s and leaves k contiguous;
  - with an anchor (kA, tsA), a whole Δ = int32(ts − tsA)/Ticks with kL − (kA + Δ) = m·N, m ≥ 1,
    is invalid and leaves the anchor where it was; RTP timestamp wrap is handled by the int32 cast;
  - no evidence — the first match, a non-whole Δ — keeps the sample and moves the anchor (Q1);
  - `loopMinMs` is the shortest t0[k+N] − t0[k] the log saw; `loopFrames` is N;
  - the sample is t1 − log[kL] on the monotonic clock.
- **Verify (done when):** `go test -race -count=1 ./internal/fingerprint/ -run 'TestSendLog|TestMatch'` exits 0
- **Parallel:** yes, with S1, S3, S5, H1
- **Repo state after:** working; still not imported

### S3 · The reassembler

- **Goal:** per viewer, packets with their arrival in, complete frames with t1 and their payload
  depacketised by a fresh pion depacketiser out; incomplete frames counted (D4, D5, D6).
- **Touches:** `internal/reassembler/reassembler.go`, `internal/reassembler/reassembler_test.go`
- **Depends on:** — (none)
- **Who:** agent
- **Proves:**
  - an in-order frame with a marker completes on the marker packet, t1 = that packet's arrival;
  - a duplicate sequence number: the first copy wins;
  - reordered packets still complete; sequence-number wrap at 65535 is handled;
  - a lost head (seq first−1 missing, or carrying the same timestamp) never completes, so it is
    never hashed;
  - a lost middle or end packet makes the frame incomplete exactly 1 s after its first packet
    (injected clock); its stale bytes never reach a hash (`H264Packet`'s FU case);
  - a stream with no marker switches to the `timestamp` rule on its first complete markerless frame,
    t1 is still the last packet's own arrival, and it never switches back; `frameEnd` reports which;
  - STAP-B, MTAP and FU-B frames come out as rejected (to be counted `unmatchedFrames`);
  - frames pending at stop are counted nowhere;
  - each frame records its completion instant (D6, for WB-41), and nothing reads it.
- **Open (carried from More research, D4):** whether 1 s outlasts pion's NACK retries — S3 makes
  the window one named constant; S7 observes it.
- **Verify (done when):** `go test -race -count=1 ./internal/reassembler/` exits 0
- **Parallel:** yes, with S1, S2, S5, H1
- **Repo state after:** working; not imported

### S4 · The clip round-trip test the Design keeps

- **Goal:** turn the 2026-10-03 offline verification (D4, More research) into a test: the clip
  through one long-lived payloader at MTU 1200, as the publisher does, then S3, then S1, gives every
  frame's table fingerprint.
- **Touches:** `internal/reassembler/roundtrip_test.go`
- **Depends on:** S1, S3
- **Who:** agent
- **Proves:** two loops of the embedded H.264 clip, 240/240 frames match, STAP-A packets included;
  two loops of the VP8 clip, 120/120; no frame is incomplete or rejected on a lossless path.
- **Verify (done when):** `go test -race -count=1 ./internal/reassembler/ -run TestRoundTrip -v` exits 0
  and its output shows both codecs: `… -v | grep -cE -- '--- PASS: TestRoundTrip.*(H264|VP8)'` prints 2
- **Parallel:** yes, with S6 (disjoint files)
- **Repo state after:** working; the pipeline is proven offline, not yet wired

### S5 · Report blocks, Markdown, stdout and the two name guards

- **Goal:** the `oneWayDelay` list of source blocks per viewer and pooled, its Method line, the
  Markdown row and stdout headline above packet transit, `view` unavailable, and the guards of D8
  (D7, D8).
- **Touches:** `internal/report/report.go`, `internal/report/markdown.go`,
  `internal/report/report_test.go`, `internal/metrics/metrics_test.go`, `cmd/whipbench/main.go`,
  `cmd/whipbench/main_test.go`
- **Depends on:** — (none: report owns its input type and is tested on synthetic viewers, as
  `report_test.go` already is)
- **Who:** agent
- **Proves:**
  - each viewer carries `oneWayDelay: [{source: "fingerprint", …}]` with the D7 keys; the pooled
    block adds `viewers`, `viewersByFrameEnd`, `duplicateFrames`, `loopFrames`, `loopMinMs`;
  - histograms merge; reasons follow `mostCommon` and "pooled over X of Y";
  - zero samples is unavailable with the counts in the reason, never a number (`CLAUDE.md` rule 2);
  - a `view` report says unavailable, "the send log lives in the `run` process";
  - the schema stays v0 and `packetTransit` is unchanged;
  - "one-way delay" occurs exactly once in a report's JSON, in the fingerprint Method line, never
    in packet transit's; `latency` stays forbidden (`TestPacketTransitKeys`, rewritten);
  - no `one_way_delay` Prometheus series, with a message that names the missing source label;
  - the Markdown table and the stdout line lead with one-way delay, packet transit below;
  - the Method line states the 1 s window, that pending frames at stop are uncounted, and that the
    CPU cost is unmeasured (D4, Impact).
- **Verify (done when):** `go test -race -count=1 ./internal/report/ ./internal/metrics/ ./cmd/whipbench/` exits 0
- **Parallel:** yes, with S1, S2, S3, H1
- **Repo state after:** working; every report says one-way delay is unavailable, with a reason,
  until S6 fills it — no fake number in between

### S6 · Wire it into `run`

- **Goal:** the runner builds one table and one send log, shared like `*metrics.Live`; the publisher
  logs t0 before the frame's first `WriteRTP`; each viewer feeds S3 from `ReadRTP` and fills its
  report block (D2, D4, D7).
- **Touches:** `internal/publisher/publisher.go`, `internal/viewer/viewer.go`,
  `internal/runner/runner.go`, `internal/runner/runner_test.go`
- **Depends on:** S1, S2, S3, S5
- **Who:** agent
- **Proves:**
  - VP8 and H.264 round trips through the test relay give an available figure, `frameEnd: marker`,
    samples > 0, `unmatchedFrames` = 0, `invalid` = 0 (`TestRoundTripVP8`, `TestRoundTripH264`
    extended);
  - a publisher with no log (`publish`) streams as before;
  - every existing runner test stays green.
- **Verify (done when):** `go test -race -count=1 ./internal/runner/ -run 'TestRoundTrip'` exits 0
- **Parallel:** yes, with S4 only
- **Repo state after:** working; the feature is exposed in `run` reports

### S7 · Markerless and lossy relays

- **Goal:** a test-relay option that clears the marker, and the lossy relay, exercise D5's
  `timestamp` rule and D4's incomplete frames end to end.
- **Touches:** `internal/testserver/testserver.go`, `internal/runner/runner_test.go`,
  `thoughts/WB-38-frame-fingerprint/99-progress.md` (the observation)
- **Depends on:** S6 (shares `runner_test.go`)
- **Who:** agent
- **Proves:**
  - through the markerless relay every viewer reports `frameEnd: timestamp` and the figure is still
    available;
  - through the lossy relay (`DropEvery`) `incompleteFrames` > 0, no sample comes from an incomplete
    frame, and the figure is available with fewer samples;
  - `TestLossThroughALossyRelay`'s existing loss assertions still hold.
- **Open (carried from More research, D4):** whether `DropEvery` yields retransmissions or only
  holes, and whether 1 s outlasts pion's NACK retries. The test asserts only what holds either way;
  Implement records which happened.
- **Verify (done when):** `go test -race -count=1 ./internal/runner/ -run 'TestMarkerless|TestLoss'` exits 0,
  and `grep -n 'DropEvery: ' thoughts/WB-38-frame-fingerprint/99-progress.md` finds the dated line
  saying "retransmissions" or "holes"
- **Parallel:** no — after S6
- **Repo state after:** working; both frame-end rules and the incomplete count are covered

### S8 · Docs, CHANGELOG and backlog

- **Goal:** the definition in its three places, the CHANGELOG entry, WB-38 closed and the Design's
  three follow-ups filed.
- **Touches:** `README.md`, `CHANGELOG.md`, `BACKLOG.md`, `ROADMAP.md` (regenerated by
  `npm run roadmap`)
- **Depends on:** S5 (Method wording), S6, S7 (WB-38 closes only when its tests have landed)
- **Who:** agent
- **Proves:**
  - the README table defines one-way delay as the Method line and the package comment do
    (`CLAUDE.md:44-45`), first-packet send to last-packet arrival;
  - `[Unreleased]` names the new key, schema v0 kept, and the two departures that change what a
    reader gets (D4, D6);
  - the backlog lints, with three new items: live one-way delay series with a source label; frame
    rate for Annex-B clips from the clip; one-way delay split by keyframe and delta frame.
- **Verify (done when):** `sh scripts/check-repo.sh && npm run backlog` exits 0, and
  `grep -l 'first-packet send to last-packet arrival' README.md internal/report/report.go internal/fingerprint/fingerprint.go | wc -l` prints 3
- **Parallel:** no — last
- **Repo state after:** working; WB-38 done

### H1 · H.264 VCL NAL types against Table 7-1

- **Goal:** close D1's open item: are the VCL NAL unit types exactly 1-5?
- **Touches:** `thoughts/WB-38-frame-fingerprint/02-design.md` (the More-research box only)
- **Depends on:** — (none)
- **Who:** human — Table 7-1 was unreachable to the research session
- **Verify (done when):** `grep -n '\[x\] D1: H.264 VCL NAL types' thoughts/WB-38-frame-fingerprint/02-design.md`
  finds the ticked, dated line; if the answer is not 1-5, the change goes into S1's predicate
  (one line plus a test case) before S8
- **Parallel:** yes, with everything
- **Repo state after:** unchanged code

### Not a WB-38 step · the live run

The first live figure — a re-run against MediaMTX with this method — is **WB-5**'s, published in
`evals/` (`BACKLOG.md:66-67`); WB-38 is done on the offline relay. D3's open item (whether RTP
timestamp intervals survive servers other than MediaMTX) is **WB-4**'s. Neither blocks a step here.

---

## Dependency graph

```
 S1 ──┬──────────────────► S4
 S3 ──┤
      └──┐
 S2 ─────┼──► S6 ──► S7 ──► S8
 S5 ─────┘
 H1 ┄┄┄► S1's predicate, only if the answer is not 1-5

 S4 needs S1, S3.   S6 needs S1, S2, S3, S5.   S7 needs S6.   S8 needs S5, S6, S7.
```

**Parallelisable:** S1 ‖ S2 ‖ S3 ‖ S5 ‖ H1 — no file in common (S1 and S2 share the
`internal/fingerprint` directory, not a file; S1 owns the package comment). Then S4 ‖ S6 —
`internal/reassembler/roundtrip_test.go` against `publisher`, `viewer`, `runner`. S7 and S8 are
sequential: S7 shares `runner_test.go` with S6, S8 closes the ticket.

---

## Recommended execution order

1. S1 ‖ S2 ‖ S3 ‖ S5 ‖ H1
2. S4 ‖ S6
3. S7
4. S8

---

## Per-step risks

| Step | Risk | Fallback |
|---|---|---|
| S1 | VCL ≠ types 1-5 (H1 open), or a server alters a VCL byte | one predicate to change; a mismatch shows as `unmatchedFrames`, never a wrong sample |
| S1 ‖ S2 | two branches create the same package; the second merge conflicts on nothing but must re-run the gate | land S1 first if both are ready; S2 rebases and re-runs |
| S2 | lock on the hot path is slow under 100 viewers | atomics on the ring; cost stays unmeasured per Impact (WB-25) |
| S2 | the first match aliases a whole loop (D3 weak spot) | documented in the Method line, not fixed here |
| S3 | the 1 s window shorter than NACK recovery (open, D4) | the constant is one line; S7's observation decides |
| S3 | session over 40% context (many cases) | split: S3a completion and t1, S3b incomplete, markerless and rejection |
| S4 | payloader state differs from the publisher's (MTU, STAP-A) | copy the publisher's payloader settings; the 2026-10-03 run used MTU 1200 |
| S5 | the guard rewrite loosens what WB-1 protected | the "exactly once, fingerprint Method only" assertion is stricter than the old one |
| S6 | three packages in one session over 40% | split: S6a publisher + runner fill the log (test: entries = frames sent), S6b viewer feeds the reassembler |
| S6 | viewer work under `pr.mu` adds read-path latency | hash outside the lock, on the completed frame |
| S7 | `DropEvery` drops retransmissions too, so no frame ever completes | assert incomplete > 0 and available-or-reasoned, record the behaviour |
| S8 | `npm run backlog` needs the network (`npx backlogsync@0.1.0`) | run where the network is; CI's backlog workflow repeats it |
| H1 | no one with the spec in hand | stays open; the predicate ships as 1-5 and the README says so |

---

## Status

- [x] Decomposition complete
- [x] Every step has a verification command
- [x] Every step leaves the repo working
- [x] Dependencies and parallelism mapped
- [ ] Approved (<date>, <who>, <how: in writing / in chat / in review>)

> Next phase: **Plan**. It receives: this file + `02-design.md`.
