# 99 · Progress — WB-1 Latency method

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
| S1 | ⏸️ blocked | S1 session | — | code complete, tests green; Verify 11 and 12 fail only on the guard-test text § S1 Tests prescribes — D1 open |
| S2 | ⬜ todo | — | — | |
| S3 | ⬜ todo | — | — | |
| S4 | ⬜ todo | — | — | |
| S5 | ⬜ todo | — | — | human — the maintainer |

Legend: ⬜ todo · 🔄 in progress · ✅ done · ⏸️ blocked · ❌ failed

---

## Where I left off

**Current step:** S1 (blocked on D1); S2/S3 run in parallel on `wb-1/s2-s3`

**Done so far:**
- S1 committed on branch `wb-1/s1` (worktree `wb1-s1`, from `origin/main` `45554d9`; no code drift from `fe24f42`, only `04-plan.md` was added since): every row of `04-plan.md` § S1 Changes applied to the 15 files of its "Files touched", plus this file

**Next concrete action:**
- the maintainer decides D1 (accept the scoped Verify 11 and 12 below, or another fix to `04-plan.md` § S1 Verify); then S1 → `✅ done`, and the S4 session merges `wb-1/s2-s3` into `wb-1/s1` and runs S4

**Modified but uncommitted files:**
- `thoughts/WB-1-latency-method/99-progress.md` — the S1 commit SHA, written after the commit

---

## Discoveries

Things found along the way that were not in the plan. **Do not fix them here** — they
go to a follow-up or a replanning round.

| # | Discovery | Path | Action |
|---|---|---|---|
| — | none in S1 | | |

---

## Deviations from the plan

Points where the plan was wrong or incomplete. Every line here signals an upstream
artifact that needs correcting.

### D1 · S1 Verify 11 and 12 contradict S1's own Tests table

- **The plan said:** § S1 Tests prescribes `TestPacketTransitKeys` with the doc comment `… no key, no definition is called latency (WB-1, D1).` and the assertions `strings.Contains(strings.ToLower(…), "latency")` and `strings.Contains(…, "one-way delay")`, and adds to `TestExposition` the assertion `strings.Contains(body, "one_way_delay")`. § S1 Verify then requires `! git grep -ni latency -- '*.go' scripts/check-repo.sh AGENTS.md` (11) and `! git grep -n -E 'one-way delay|one_way_delay|…' -- '*.go'` (12) to exit 0.
- **Reality is:** the guards must name what they guard against, so 11 and 12 can never pass. Their only hits are those four prescribed lines: `internal/report/report_test.go:103` and `:114` (11), `internal/metrics/metrics_test.go:41` and `internal/report/report_test.go:117` (12). No other `.go` file, `scripts/check-repo.sh` or `AGENTS.md` matches.
- **What I did:** applied the plan literally, did not edit the plan or the tests, and committed. I also ran two scoped forms, which exit 0: `! git grep -ni latency -- '*.go' scripts/check-repo.sh AGENTS.md ':!internal/report/report_test.go'` and `! git grep -n -E '<same pattern>' -- '*.go' ':!internal/report/report_test.go' ':!internal/metrics/metrics_test.go'`. The same is true when the hits are piped through `grep -v` on the four guard lines.
- **Artifact to fix:** `04-plan.md` § S1 Verify, lines 11 and 12: scope out the guard lines.
- **Re-enter:** none, because the intent is intact and only the check's scope is wrong.
- **Landed steps:** S1 keep.
- **Status:** open. The maintainer must approve the scoped commands before S1 is marked done.

---

## Verifications run

| Command | When | Result |
|---|---|---|
| 1 `gofmt -w cmd internal` | S1 | exit 0 |
| 2 `go build ./... && go vet ./... && test -z "$(gofmt -l .)"` | S1 | exit 0 |
| 3 `go test -count=1 -run 'TestPacketTransit\|TestExposition\|TestNilLiveIsANoOp' ./internal/report/ ./internal/metrics/` | S1 | ok report, ok metrics; exit 0 |
| 4 `go test -race -count=1 -run 'TestRoundTrip\|TestStrippedExtension' ./internal/runner/` | S1 | ok runner; exit 0 |
| 5 `go test ./... && go vet ./... && test -z "$(gofmt -l .)" && ./scripts/check-repo.sh && node scripts/leakcheck.mjs` | S1 | all packages ok; `ok — repo invariants hold at 0.0.1`; leakcheck `nothing private in 71 tracked files, 23 commits` (no-denylist notice); exit 0 |
| 6 `golangci-lint run ./...` | S1 | `0 issues.`; exit 0 |
| 7 `grep -q 'packet transit: unavailable' README.md && grep -q 'not glass-to-glass' README.md` | S1 | exit 0 |
| 8 `grep -q 'whipbench_packet_transit_seconds' internal/metrics/metrics.go` | S1 | exit 0 |
| 9 `grep -q 'packetTransit' internal/viewer/viewer.go` | S1 | exit 0 |
| 10 `sed -n '/^## \[Unreleased\]/,/^## \[/p' CHANGELOG.md \| grep -w 'WB-1' \| grep -qi 'packet transit'` | S1 | exit 0 |
| 11 `! git grep -ni latency -- '*.go' scripts/check-repo.sh AGENTS.md` | S1 | **exit 1**: hits `report_test.go:103`, `:114` only, both plan-prescribed (D1) |
| 12 `! git grep -n -E 'one-way delay\|one_way_delay\|MaxPlausibleDelay\|DelayBucketsMs\|\.Delay\(\|finishLatency' -- '*.go'` | S1 | **exit 1**: hits `metrics_test.go:41`, `report_test.go:117` only, both plan-prescribed (D1) |
| 13 `! grep -n -E 'latency or why not\|\*\*Latency is' CLAUDE.md` | S1 | exit 0 |
| 11 and 12 scoped (D1) | S1 | exit 0, exit 0 |
| extra `go test -race -count=1 ./...` | S1 | exit 0 |
| extra `node scripts/leakcheck_test.mjs` | S1 | exit 0 |
| extra `git diff --stat origin/main` | S1 | the 15 files of § S1 "Files touched" and nothing else (142+/112−); `99-progress.md` is new |

---

## Context budget

| Session | Step | Peak context | Note |
|---|---|---|---|
| S1 session | S1 | about 6% | read only the plan's References, Minimum context and § S1 |

---

## Observations

Human observations the plan's steps close on (S5). One line each.
