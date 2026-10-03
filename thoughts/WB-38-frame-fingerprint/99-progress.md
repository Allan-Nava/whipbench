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
| S1 | ⬜ todo | — | — | fingerprint + clip table, `isVCL` predicate |
| S2 | ⬜ todo | — | — | send log ring + matcher (D3 aliasing) |
| S3 | ⬜ todo | — | — | reassembler; may split S3a/S3b |
| S4 | ⬜ todo | — | — | offline clip round trip; after S1, S3 |
| S5 | ⬜ todo | — | — | report blocks, Markdown, stdout, guards |
| S6 | ⬜ todo | — | — | wire into `run`; after S1, S2, S3, S5; may split S6a/S6b |
| S7 | ⬜ todo | — | — | markerless + lossy relays; records the `DropEvery:` observation |
| S8 | ⬜ todo | — | — | docs, CHANGELOG, backlog; last |
| H1 | ⬜ todo | — | — | maintainer: VCL types vs H.264 Table 7-1 |

Legend: ⬜ todo · 🔄 in progress · ✅ done · ⏸️ blocked · ❌ failed

---

## Where I left off

**Current step:** S1 ‖ S2 ‖ S3 ‖ S5 (first wave, in parallel; the merging session owns this file and copies each step's row in from its PR)

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
| 1 | | | follow-up / replan / ignore |

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
