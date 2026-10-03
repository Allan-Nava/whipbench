# 99 · Progress — WB-38 One-way delay by frame fingerprint

> Shared state across Implement sessions. **Update it before closing every session** —
> or, with sessions in parallel, report to the one owner who does (`05-implement.md`).
> This is the intra-phase compaction artifact: when context passes 40%, this file is
> all that carries over to the next session.
>
> It must be self-contained: explicit paths, no reference to session context.

---

## Step status

| Step | Status | Session | Commit | Note |
|---|---|---|---|---|
| S1 | ✅ done | S1 session | `55ad054 (#68)` | fingerprint + clip table, isVCL predicate; 8 plan tests, gate green |
| S2 | ✅ done | S2 session | `494f759 (#69, rebased on S1)` | send log ring + matcher (D3 aliasing); 17 plan tests, gate green after rebase; discovery: Matcher needs ticks > 0 |
| S3 | ✅ done | S3 session | `32bfcf7 (#70)` | reassembler + 14 tests, not split; state bounded (max seen 261, closed 91, pending 5); after the marker→timestamp switch a frame ends on the next later-timestamp packet whatever its markers (plan 2.6(c) wording, its expected value followed); discovery: a duplicate > 2·Window late reopens a frame |
| S4 | ⬜ todo | — | — | offline clip round trip; after S1, S3 |
| S5 | ✅ done | S5 session | `6ca4c4b (#71)` | report blocks, Markdown, stdout, guards; discoveries: joined viewer without a fingerprint block counts as "had none" (revisit with WB-39), FingerprintDelay fallback sets an empty ViewersByFrameEnd, frameEnds helper |
| S6 | ⬜ todo | — | — | wire into `run`; after S1, S2, S3, S5; may split S6a/S6b |
| S7 | ⬜ todo | — | — | markerless + lossy relays; records the `DropEvery:` observation |
| S8 | ⬜ todo | — | — | docs, CHANGELOG, backlog; last |
| H1 | ⬜ todo | — | — | maintainer: VCL types vs H.264 Table 7-1 |

Legend: ⬜ todo · 🔄 in progress · ✅ done · ⏸️ blocked · ❌ failed

---

## Where I left off

**Current step:** S4 ‖ S6 (second wave). First wave merged 2026-10-03: S1 #68, S2 #69 (rebased on S1, gate re-run green), S3 #70, S5 #71.

**Done so far:**
- <what has been written, with paths>

**Next concrete action:**
- <the exact next executable step>

**Modified but uncommitted files:**
- `<path>` — <what>

---

## Discoveries

Things found along the way that were not in the plan. **Do not fix them here** — they
go to a follow-up or a replanning round.

| # | Discovery | Path | Action |
|---|---|---|---|
| 1 | `NewMatcher` with `ticks == 0` panics later at `dts % ticks` in `Match`; the plan adds no guard (S2) | `internal/fingerprint/match.go` | S6 must never pass a clip whose `Ticks` is 0 — told to S6 |
| 2 | A duplicate packet more than 2·Window after the original reopens a frame, which later expires as incomplete; no test covers it (S3) | `internal/reassembler/reassembler.go` | follow-up — needs a relay retransmitting > 2 s late |
| 3 | After the marker→timestamp switch, plan behaviour 2.6(c) read literally keeps a frame with a marker from closing; S3 followed the plan's expected value instead (a frame ends on the next later-timestamp packet) | `04-plan.md` § S3 2.6(c) | replan wording if the step is ever re-run |
| 4 | A joined viewer without a fingerprint block counts as "had none" with a "no valid sample" reason (S5) | `internal/report/report.go` | revisit with WB-39's second source |
| 5 | The plan's shared gate omits `./scripts/check-repo.sh`, which CI runs (S2) | `04-plan.md` § Minimum context | every step ran it anyway |

---

## Deviations from the plan

Points where the plan was wrong or incomplete. Every line here signals an upstream
artifact that needs correcting.

### D<n> · <title>

- **The plan said:** <...>
- **Reality is:** <...> (`path:line`)
- **What I did:** stopped / deviated with approval / <...>
- **Artifact to fix:** <`04-plan.md` § Sn, or the upstream artifact>
- **Re-enter:** none / Structure / Design / Research / Questions — see `recovery.md`
- **Landed steps:** <per landed step: keep / adapt / revert (`<sha>`)>
- **Status:** <open / resolved> — <corrected artifact § entry, commit>

---

## Verifications run

| Command | When | Result |
|---|---|---|
| `<command>` | <step> | <result> |

---

## Context budget

| Session | Step | Peak context | Note |
|---|---|---|---|
| <session> | <step> | <peak context> | |
