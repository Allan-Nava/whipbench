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
| S1 | ✅ done | S1 session | `e0318e0` | code complete, tests green; Verify 11 and 12 scoped by D1 (resolved 2026-10-02), exit 0 |
| S2 | ✅ done | S2/S3 session | `8bcee20` | WB-38 to WB-41 written on `wb-1/s2-s3`; slip in the "Ids first" grep fixed (D2) |
| S3 | ✅ done | S2/S3 session | `b5fdc6c` | WB-2, WB-3, WB-4 rewritten, citing lines follow, on `wb-1/s2-s3` |
| S4 | ✅ done | S4 session | `df16299` | `wb-1/s2-s3` merged at `e639f09`; README `:7`, the "Latency, and its limits" section and `:146` rewritten as § S4 Changes says |
| S5 | ✅ done | maintainer | — | human — approved in chat on the PR summary, 2026-10-02 |

Legend: ⬜ todo · 🔄 in progress · ✅ done · ⏸️ blocked · ❌ failed

---

## Where I left off

**Current step:** S5 (the maintainer); S1-S4 done, PR open, not merged

**Done so far:**
- S1 committed on branch `wb-1/s1` (worktree `wb1-s1`, from `origin/main` `45554d9`; no code drift from `fe24f42`, only `04-plan.md` was added since): every row of `04-plan.md` § S1 Changes applied to the 15 files of its "Files touched", plus this file

- D1 resolved (2026-10-02, Allan Nava, in chat): `04-plan.md` § S1 Verify 11 and 12 scoped, both exit 0; D2 slip fixed in `04-plan.md` § S2 "Ids first"
- S2 (`8bcee20`) and S3 (`b5fdc6c`) done on `wb-1/s2-s3`, recorded from the S2/S3 session's report

- S4: `wb-1/s2-s3` merged into `wb-1/s1` (`e639f09`), README commit `df16299`; S4 Verify and S1 Verify (scoped) all exit 0 on the merged branch; `wb-1/s1` pushed and opened as the one PR

**Next concrete action:**
- the maintainer reads the PR and appends the S5 line under `## Observations` (`04-plan.md` § S5), then the PR merges with `gh pr merge --squash`

**Modified but uncommitted files:**
- none

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
- **Status:** resolved 2026-10-02, Allan Nava, in chat: § S1 Verify line 11 now excludes `':!internal/report/report_test.go'` and line 12 excludes `':!internal/report/report_test.go' ':!internal/metrics/metrics_test.go'` in `04-plan.md`, because the guard tests § S1 Tests prescribes must contain the forbidden words. Both scoped lines re-run on `wb-1/s1`: `exit 0`, `exit 0`. S1 → `✅ done`.

### D2 · S2 "Ids first" grep counts the format example (transcription slip, fixed)

- **The plan said:** `04-plan.md:336`, `grep -oE '\*\*WB-[0-9]+ — ' BACKLOG.md | grep -oE '[0-9]+' | sort -n | tail -1` prints `37`.
- **Reality is:** it prints `99`, because it matches the `WB-99` format example at `BACKLOG.md:15`. The highest real id is WB-37 (`BACKLOG.md:183`), so WB-38 to WB-41 stand (S2/S3 session report).
- **What I did:** fixed the plan line as a transcription slip: `| grep -v '^99$'` added to the pipeline. It sits before `sort -n`, not after `tail -1`, because after `tail -1` it would drop the only line and print nothing. Re-run against `origin/main:BACKLOG.md`: prints `37`, exit 0.
- **Re-enter:** none. **Landed steps:** S2 keep.
- **Status:** fixed.

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
| 11 and 12, scoped in `04-plan.md` (D1 resolved) | S4 session, on `wb-1/s1` before the merge | exit 0, exit 0 |
| S4 Verify, the whole block (`npm ci` … `go test ./... && npm run backlog`) | S4 session, on `wb-1/s1` after `df16299` | check-repo `ok — repo invariants hold at 0.0.1`, leakcheck `nothing private in 72 tracked files, 27 commits` (no-denylist notice), site built (12 sections); every `exit` line exit 0 (6 of 6); `for` line printed nothing; backlog `41 items, 8 milestones` |
| S1 Verify, the whole block with 11 and 12 scoped | S4 session, on merged `wb-1/s1` after `df16299` | every `exit` line exit 0 (12 of 12); `golangci-lint` `0 issues.`; `gofmt -w` changed nothing |
| S2 Verify | S2/S3 session, after `8bcee20` | roadmap + `git diff --exit-code ROADMAP.md` exit 0 (41 items, 8 milestones); backlog + leakcheck exit 0; four new ids, placement (WB-38 v0.1.0, WB-39..41 v0.2.0) and content checks exit 0; diff BACKLOG.md +52, ROADMAP.md +7 −3 only |
| S3 Verify | S2/S3 session, after `b5fdc6c` | roadmap exit 0 (41 items, 8 milestones); backlog + leakcheck exit 0; WB-2/WB-3/WB-4 content checks and citing-phrase checks exit 0; diff BACKLOG.md +44 −26, ROADMAP.md +5 −5 only; S2 Verify re-run after S3 all exit 0 |

---

## Context budget

| Session | Step | Peak context | Note |
|---|---|---|---|
| S1 session | S1 | about 6% | read only the plan's References, Minimum context and § S1 |
| S4 session | D1, S2/S3 record, S4, landing | about 4% | read the plan's References, Minimum context, § S1 Verify, § S4, § S5 format, and the S2/S3 report |

---

## Observations

- S5 observed: 2026-10-02, Allan Nava: approved in chat ("ok continua") after a summary of PR #48 — what S1–S4 change, the breaking rename, WB-38..41 and the rewritten WB-2/3/4; reading the README section and the new items in full was not stated, so this is an approval of the summary
Human observations the plan's steps close on (S5). One line each.
