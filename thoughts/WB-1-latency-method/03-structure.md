# 03 · Structure — WB-1 Latency method

**Written against:** `1098ca8`

---

## Reference

Design: [`02-design.md`](./02-design.md) — approved 2026-10-02.

**Scope (the Design's Q3 default, accepted):** WB-1 decides; it does not implement. Two kinds of
step only: the one code change D1 allows (the 0.0.1 metric renamed "packet transit"), and the
records the decision produces (backlog items, ROADMAP, README, CHANGELOG). D2-D8 are built by
the follow-up items S2 and S3 write, not here.

**Constraints carried from the Design's Review table** — each lands in a named step and is
checked by its Done-when:

- for H.264, D4 hashes **VCL NAL units only** (a server may add or repeat SPS/PPS) — S2, in WB-38;
- D4's **4 s clip-loop limit** is stated in the report — S2, in WB-38's done-when;
- the remaining **More research needed** items stay open — each is carried, unticked, into the
  item it blocks: D4 other servers' byte-identity → WB-4 (S3); D3 NACK resend keeps extensions →
  WB-39 (S2); D7 NACK count semantics → WB-41 (S2); D4/D5 CPU per viewer → WB-38 (S2); D5 code
  survives libvpx at 600 kbit/s → WB-2 (S3).

**Ids.** The highest id in `BACKLOG.md` at `1098ca8` is WB-37 (`WB-99` at `BACKLOG.md:15` is the
format example, not an item), so the four new items are **WB-38** one-way delay by frame
fingerprint, **WB-39** abs-capture-time once per frame, **WB-40** topology, clock and
comparability, **WB-41** sample window and retransmission — the Design's order. If `main` gains
an item before S2 lands, S2 renumbers from the new highest and every later step follows.

**Notation.** `item WB-n` in a Done-when prints that item's lines, from its bullet to the next
bullet or heading; define it once in the shell before running one:

```sh
item() { awk -v id="**$1 — " 'index($0,id){p=1;print;next} /^- \[|^## /{p=0} p' BACKLOG.md; }
```

All commands run from the repository root. `./scripts/check-repo.sh`, `go vet ./...` and the
`gofmt -l .` test are what `.github/workflows/ci.yml` runs beside `go test`.

---

## Steps

### S1 · Rename the 0.0.1 metric to "packet transit" (D1)

- **Goal:** the per-packet send-stamp figure is called "packet transit" wherever whipbench names
  it, in one commit with tests green; no metric is called "latency" (D1).
- **Touches:** `README.md` (`:53`, the "Latency, and its limits" section `:60-71` — phrases only),
  `scripts/check-repo.sh` (`:16`, phrase and message), `internal/report/report.go` (`:104`,
  `:290`), `internal/report/markdown.go` (`:58`, `:77`), `internal/viewer/viewer.go` (`:55-68`,
  `:92`), `internal/metrics/metrics.go` (`:19`, `:93-94`), `cmd/whipbench/main.go` (`:272`),
  `internal/report/report_test.go`, `internal/runner/runner_test.go`,
  `internal/metrics/metrics_test.go`, `CHANGELOG.md` (one `[Unreleased]` line citing WB-1)
- **Depends on:** — (none)
- **Who:** agent
- **Done when:** each of these exits 0:
  `go test ./... && go vet ./... && test -z "$(gofmt -l .)" && ./scripts/check-repo.sh && node scripts/leakcheck.mjs`;
  `grep -q 'packet transit: unavailable' README.md && grep -q 'not glass-to-glass' README.md`;
  `grep -q 'whipbench_packet_transit_seconds' internal/metrics/metrics.go`;
  `grep -q 'packetTransit' internal/viewer/viewer.go`;
  `sed -n '/^## \[Unreleased\]/,/^## \[/p' CHANGELOG.md | grep -w 'WB-1' | grep -qi 'packet transit'`;
  `! git grep -ni latency -- '*.go' scripts/check-repo.sh` (no `-w`: `whipbench_latency_seconds`
  must match too)
- **Parallel:** yes, with S2, and with S3 once S2 is in — no shared file.
- **Repo state after:** working; reports, Prometheus and stdout say "packet transit"; schema
  stays `whipbench.report/v0`; one-way delay not yet measured.

### S2 · Write the four new follow-up items, WB-38 to WB-41

- **Goal:** the Design's four new follow-up items are in `BACKLOG.md` with real ids, each citing
  its decisions and carrying the open More-research facts it rests on; ROADMAP regenerated.
- **Touches:** `BACKLOG.md`, `ROADMAP.md` (regenerated with `npm run roadmap`, never hand-edited)
- **Depends on:** — (none)
- **Who:** agent
- **Done when:** `npm run roadmap && git diff --exit-code ROADMAP.md` after the commit, and
  `npm run backlog && node scripts/leakcheck.mjs` exit 0, and each of these greps matches:
  `test "$(grep -cE '\*\*WB-(38|39|40|41) — ' BACKLOG.md)" = 4`;
  `item WB-38 | grep -q 'D4' && item WB-38 | grep -q 'VCL NAL' && item WB-38 | grep -q '4 s'`
  (WB-38 = D2 + D4 in `run`, every viewer, WB-5's headline; H.264 hashes VCL NAL units only; the
  report states the 4 s clip-loop limit; CPU per viewer unmeasured);
  `item WB-39 | grep -q 'D3' && item WB-39 | grep -qi 'packet transit' && item WB-39 | grep -q 'NACK'`
  (retires packet transit; the NACK-resend-keeps-extensions fact is open);
  `item WB-40 | grep -q 'D6' && item WB-40 | grep -q 'D8'`;
  `item WB-41 | grep -q 'D7' && item WB-41 | grep -q 'excludeFirstSeconds' && item WB-41 | grep -q 'NACK'`
- **Parallel:** yes, with S1 — no shared file. Not with S3 (same two files).
- **Repo state after:** working; `npm run backlog` green; WB-2/WB-3/WB-4 still say what they said.

### S3 · Rewrite WB-2, WB-3 and WB-4

- **Goal:** WB-3 becomes the clock exchange (D6), WB-2 the baked-index keyframe
  capture-to-decode for VP8, moved to v0.6.0 (D5), WB-4 widened to payload byte-identity and the
  marker bit per server (D4, D2); every other backlog line that cites WB-2 as glass-to-glass or
  WB-3 as the delay method is brought in line.
- **Touches:** `BACKLOG.md`, `ROADMAP.md` (regenerated)
- **Depends on:** S2 — the rewritten items cite WB-38 to WB-41 (WB-2 keys capture-to-decode on
  WB-38's send log; WB-3 feeds WB-40's `clock` block), and both steps edit the same two files.
- **Who:** agent
- **Done when:** `npm run roadmap && git diff --exit-code ROADMAP.md` after the commit,
  `npm run backlog && node scripts/leakcheck.mjs` exit 0, and:
  `awk '/^## /{h=$0} index($0,"**WB-2 — "){print h}' BACKLOG.md | grep -q 'v0.6.0'`;
  `item WB-2 | grep -q 'capture-to-decode' && item WB-2 | grep -q 'libvpx'` (the open D5 fact);
  `! grep -q 'Visual timestamp for glass-to-glass' BACKLOG.md`;
  `item WB-3 | grep -q -- '--clock-peer' && item WB-3 | grep -q 'publish'`;
  `item WB-4 | grep -qi 'marker' && item WB-4 | grep -qi 'byte'`
- **Parallel:** with S1 only — no shared file. Not with S2.
- **Repo state after:** working; the backlog carries the whole decision.

### S4 · Point the README at the decision

- **Goal:** the README's "Latency, and its limits" section and its other WB-1/WB-3 mentions
  (`README.md:7`, `:67`, `:71`, `:146`) say what WB-1 decided and where each method is built
  (WB-38 to WB-41, WB-2, WB-3), keeping S1's phrases and "not glass-to-glass".
- **Touches:** `README.md`
- **Depends on:** S1 (same section, its phrases are what `check-repo.sh` checks) and S3 (the ids
  and the rewritten WB-2/WB-3 it cites).
- **Who:** agent
- **Done when:** `./scripts/check-repo.sh && node scripts/leakcheck.mjs && npm run build:site`
  exit 0, and
  `sed -n '/^## Latency, and its limits/,/^## [^L]/p' README.md | grep -q 'WB-38'` and
  `sed -n '/^## Latency, and its limits/,/^## [^L]/p' README.md | grep -q 'one-way delay'`
- **Parallel:** no — last of the agent steps.
- **Repo state after:** working; README, backlog and code agree.

### S5 · The maintainer reads the result

- **Goal:** Allan Nava reads the renamed README section, the four new items and the three
  rewritten ones in the PR, and records that they say what the Design decided.
- **Touches:** `thoughts/WB-1-latency-method/99-progress.md` (the observation line only)
- **Depends on:** S4
- **Who:** human — observation recorded in `thoughts/WB-1-latency-method/99-progress.md` as a line
  `- S5 observed: YYYY-MM-DD, name: what was read and whether it says what the Design decided`
- **Done when:** `grep -nE '^- S5 observed: [0-9]{4}-[0-9]{2}-[0-9]{2}, ' thoughts/WB-1-latency-method/99-progress.md`
  prints one line
- **Parallel:** no.
- **Repo state after:** working; WB-1 can close.

---

## Dependency graph

```
S1  rename (code, README phrases, check-repo, CHANGELOG) ──────────┐
                                                                   ├──► S4  README ──► S5  human read
S2  WB-38..WB-41 (BACKLOG, ROADMAP) ──► S3  WB-2/3/4 (same files) ─┘
```

**Parallelisable:** S1 ‖ S2, and S1 ‖ S3 — S1's Touches (`README.md`, `scripts/check-repo.sh`,
`internal/**`, `cmd/whipbench/main.go`, `CHANGELOG.md`) share no file with S2/S3's (`BACKLOG.md`,
`ROADMAP.md`), so S1 runs on its own worktree while S2 then S3 run on another. S2 and S3 are
sequential (same files, and S3 cites S2's ids); S4 waits for both branches (it edits S1's README
section and cites S3's rewritten items); S5 waits for S4.

---

## Recommended execution order

1. S1 ‖ S2 — two worktrees, S1 the only code change, S2 records only
2. S3 — on S2's worktree, after S2's commit
3. S4 — after S1 and S3 are both on the branch
4. S5 — the maintainer, on the PR that carries S1-S4

---

## Per-step risks

| Step | Risk | Fallback |
|---|---|---|
| S1 | A 0.0.1 dashboard or parser of `latency` meets a missing key, not an error (D1's weak spot) | intended by Q3: the CHANGELOG line says so; no shim, no dual-emit (D1 rejected both) |
| S1 | `scripts/check-repo.sh:16` and the README phrase drift apart — CI fails | both are in S1's Touches, one commit; `./scripts/check-repo.sh` is in its Done-when |
| S1 | `git grep -ni latency` also hits Go comments about the concept, not the metric | rename those too ("delay", "packet transit"); the grep stays — a narrower one would let a metric name through |
| S1 | `CLAUDE.md`, `AGENTS.md` or `.github/workflows/release-drift.yml` name the metric too (each mentions "latency") | Plan reads them; any that name the 0.0.1 metric join S1's Touches — none is in S2/S3's, so parallelism holds |
| S1 | A golden file under `testdata/` or `cmd/whipbench/main_test.go` carries the old key | Plan adds it to S1's Touches; `go test ./...` catches it |
| S2 | `main` gains a WB-38 before S2 lands, so the ids collide | renumber from the new highest (Reference, Ids); S3/S4's greps follow |
| S2 | Which milestone holds WB-38 to WB-41 is not in the Design (only "WB-5's headline" for WB-38) | Plan places them, WB-38 no later than WB-5's milestone; `npm run backlog` checks the format, not the choice — S5 reads it |
| S2, S3 | `.github/workflows/backlog-issues.yml` turns backlog items into public issues on merge — a wrong item becomes a wrong issue | S5 reads the items before merge; `node scripts/leakcheck.mjs` runs on every step |
| S3 | Another backlog line still describes WB-2 as glass-to-glass or WB-3 as the delay method (`BACKLOG.md` cites both in several places) | the Done-when greps the old WB-2 title; Plan lists every citing line before editing |
| S3 | v0.1.0's milestone title calls the method "latency" | out of D1's rename (a milestone, not a metric); Plan decides, and records the choice |
| S4 | Renaming the "Latency, and its limits" heading would change the site's anchor and S4's own `sed` range | keep the heading (it names the topic, not a metric); if Plan renames it, the Done-when range follows |
| S5 | The maintainer is not available, so WB-1 cannot close | S1-S4 merge without S5; S5's grep stays the closing gate |

---

## Status

- [x] Decomposition complete
- [x] Every step has a verification command
- [x] Every step leaves the repo working
- [x] Dependencies and parallelism mapped
- [x] Approved (2026-10-02, Allan Nava, in chat: "ok approvo") — with the three open questions decided: WB-38 goes in v0.1.0 (it unblocks WB-5) and WB-39..41 in v0.2.0; the v0.1.0 milestone title and the README heading "Latency, and its limits" stay, since both name the subject rather than a metric

> Next phase: **Plan**. It receives: this file + `02-design.md`.
