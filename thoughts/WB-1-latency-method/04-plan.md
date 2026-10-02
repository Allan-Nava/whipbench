# 04 · Plan — WB-1 Latency method

**Written against:** `fe24f42`

---

## References

- Structure: [`03-structure.md`](./03-structure.md)
- Design: [`02-design.md`](./02-design.md)

---

## Minimum context for the executor

Everything the executor needs to know so it never has to explore. Line numbers are at
`fe24f42`; where an earlier step shifts them, the step anchors on text instead.

| Fact | Where |
|---|---|
| WB-1 decides and does not implement: one code change (the rename, S1), the rest is records (backlog, ROADMAP, README, CHANGELOG) | `03-structure.md`, Reference |
| Decisions D1-D8 the backlog items cite by number | `02-design.md`, Decisions |
| The 0.0.1 figure is per packet: arrival minus the abs-capture-time send stamp; it becomes **packet transit**, JSON key `packetTransit`, Prometheus `whipbench_packet_transit_seconds`. "One-way delay" is reserved for WB-38's per-frame figure; no metric is called "latency" | D1, `02-design.md` |
| The report schema stays `whipbench.report/v0` — do not edit it | `internal/report/report.go:30-31` |
| CI runs `gofmt -l .` (must be empty), `go vet ./...`, `go test -race -count=1 ./...`, `./scripts/check-repo.sh`, `node scripts/leakcheck_test.mjs`, `node scripts/leakcheck.mjs`, plus the backlog check and the site build | `.github/workflows/ci.yml:27-39`, `:74-75`; `AGENTS.md:11-17` |
| `scripts/check-repo.sh` greps README for `not glass-to-glass` (`:15`) and, today, `latency: unavailable` (`:16`) — a README edit that drops either phrase fails CI | `scripts/check-repo.sh:14-18` |
| `node scripts/leakcheck.mjs` scans tracked files and `git log --all -p`: no home-directory path, no e-mail but the author's public one, no tool-attribution trailer or footer. A missing private denylist is a notice, not a failure | `scripts/leakcheck.mjs:1-25`, `:43` |
| Backlog items are parsed by backlogsync 0.1.0: an item is `- [ ] **WB-n — Title**: body` plus **indented** continuation lines; a blank or unindented line ends it. The title is everything up to the first `**`. The meta comment `<!-- wb: prio=… size=… labels=… -->` sits at the end of the item. `prio` ∈ `high, med, low`; `size` ∈ `S, M, L, XL`; an open item carries no `ver=` | `BACKLOG.md:10-23`; backlogsync `bin/lib/backlog.mjs` `parse()`/`lint()` |
| Allowed labels (`package.json#backlogsync.labels`): `client`, `measurement`, `benchmark`, `report`, `research`, `release`, `docs`, `project`, `tests`, `enhancement` | `package.json#backlogsync.labels` |
| `npm run backlog` fails when `ROADMAP.md` is stale; `npm run roadmap` regenerates it; never hand-edit `ROADMAP.md` | `package.json` `scripts`; `CLAUDE.md:113-114` |
| On merge to `main`, `.github/workflows/backlog-issues.yml` syncs `BACKLOG.md` to public GitHub issues: a new item creates an issue, a changed title retitles it, a moved item moves milestone. Nothing removes an issue | backlogsync `bin/lib/plan.mjs:24-36` |
| Highest backlog id at `fe24f42` is WB-37 (`WB-99` at `BACKLOG.md:15` is the format example) | `BACKLOG.md:183` |
| Milestone placement (maintainer, Structure approval): WB-38 in v0.1.0; WB-39, WB-40, WB-41 in v0.2.0. The v0.1.0 milestone title and the README heading "Latency, and its limits" stay as they are | `03-structure.md`, Status |
| `CLAUDE.md` names the metric at `:22` and `:54-55`, `AGENTS.md` at `:5` — both join S1. `CLAUDE.md:82-83` and `README.md:113-114` describe *other tools'* latency and stay. `.github/workflows/release-drift.yml:12` says "before the latency method is decided" — the WB-1 subject, not the metric — and stays | read for this plan |
| History stays: the CHANGELOG `[0.0.1]` section, the shipped backlog items WB-9..WB-14, `evals/` (dated runs and their raw reports, whose JSON carries the old `latency` key) and `thoughts/` are records and are not edited | `CHANGELOG.md:24-45`, `BACKLOG.md:187-213` |
| The CHANGELOG has `## [Unreleased]` → `### Changed` at `:6-8`; new lines go first under `### Changed` | `CHANGELOG.md:6-12` |
| There is no viewer unit test file; the viewer's packet transit is covered end to end by `internal/runner/runner_test.go` against the in-process relay | `internal/viewer/` |

**Conventions to respect:** English, British-leaning spelling, em-dashes, no marketing
filler, no decorative emoji (`CLAUDE.md:116-117`). Go: `gofmt` decides alignment — run
`gofmt -w` on every Go file you touch rather than aligning by hand. Commits carry **no**
tool-attribution trailer or footer of any kind, and the author e-mail is the repository's
configured public one (`git config user.email`). Every command runs from the repository
root of the step's worktree.

**Branches and worktrees.** The Structure runs S1 ‖ (S2 → S3), then S4, on one pull
request. Set it up once, from the main checkout (`R` is its root; pick any scratch
directory for `WT`; `mktemp -d` gives one):

```sh
R=$(git rev-parse --show-toplevel); WT=$(mktemp -d)
git -C "$R" fetch origin
git -C "$R" worktree add -b wb-1/s1    "$WT/wb1-s1"    origin/main   # S1
git -C "$R" worktree add -b wb-1/s2-s3 "$WT/wb1-s2-s3" origin/main   # S2 then S3
```

S4 runs on `wb-1/s1` after `git -C "$WT/wb1-s1" merge --no-ff wb-1/s2-s3` (no shared
file, so the merge is clean), and that branch is pushed and opened as the one PR. The
session that runs S1 owns `99-progress.md`; the S2/S3 session does not touch it and
reports its rows (status, commit, verification results) to the owner.

**The `item` helper** used in S2 and S3 prints one backlog item, from its bullet to the
next bullet or heading. Define it in the shell first:

```sh
item() { awk -v id="**$1 — " 'index($0,id){p=1;print;next} /^- \[|^## /{p=0} p' BACKLOG.md; }
```

---

## S1 · Rename the 0.0.1 metric to "packet transit" (D1)

Worktree `wb-1/s1`. One commit: `WB-1 S1: the 0.0.1 metric is packet transit`.

**Do first:** create `thoughts/WB-1-latency-method/99-progress.md` (see "99-progress.md"
below) and mark S1 `🔄 in progress`.

**Identifier rule (the decision this step makes).** Every identifier on the 0.0.1 path
that says `Latency` or `Delay` is renamed, not only the strings: D2 will add a per-frame
one-way delay with its own type, recorder and histogram, and a `Delay` left on the
packet-transit path would be D1's rejected "one name, two meanings". The generic noun
"delay" in prose that names no metric stays (`viewer.go:349`; `runner_test.go:98`,
`:169`; `stats_test.go:36`; all of `internal/rtpstats/`, whose "transit" is RFC 3550's and
unrelated). No behaviour, no number, no reason string changes meaning.

### Files touched

| Path | Action |
|---|---|
| `thoughts/WB-1-latency-method/99-progress.md` | new |
| `internal/viewer/viewer.go` | modify |
| `internal/metrics/metrics.go` | modify |
| `internal/report/report.go` | modify |
| `internal/report/markdown.go` | modify |
| `cmd/whipbench/main.go` | modify |
| `internal/publisher/publisher.go` | modify (package comment) |
| `internal/stats/stats.go` | modify (package comment) |
| `internal/report/report_test.go` | modify + one new test |
| `internal/metrics/metrics_test.go` | modify + one new assertion |
| `internal/runner/runner_test.go` | modify |
| `scripts/check-repo.sh` | modify |
| `README.md` | modify (phrases only) |
| `CLAUDE.md` | modify (`:22`, `:54-55`) |
| `AGENTS.md` | modify (`:5`) |
| `CHANGELOG.md` | modify (one `[Unreleased]` entry) |

No golden file under `testdata/` and nothing in `cmd/whipbench/main_test.go` carries the
old key or series (checked: `git grep -n -i 'latency\|one_way'` hits neither).

### Changes

#### `internal/viewer/viewer.go`

| Line | Old → new |
|---|---|
| `:3` | comment `… — one-way delay.` → `… — packet transit.` |
| `:32` | comment `// MaxPlausibleDelay bounds a one-way delay sample.` → `// MaxPlausibleTransit bounds a packet transit sample.` (rest of the comment, `:33-34`, unchanged) |
| `:35` | `const MaxPlausibleDelay` → `const MaxPlausibleTransit` (value unchanged, 60 s) |
| `:38` | comment `implausible delay before the viewer reports latency as unavailable.` → `implausible transit before the viewer reports packet transit as unavailable.` |
| `:54` | comment `// Latency is the one-way delay a viewer measured, or why it could not.` → `// PacketTransit is a viewer's per-packet arrival minus send stamp, or why it could not be measured.` |
| `:55` | `type Latency struct` → `type PacketTransit struct` (fields and JSON tags inside unchanged) |
| `:57` | comment `why latency is unavailable` → `why packet transit is unavailable` |
| `:62` | comment `delay was negative or above MaxPlausibleDelay` → `transit was negative or above MaxPlausibleTransit` |
| `:70` | comment `the viewer's delay histogram` → `the viewer's packet transit histogram` |
| `:71` | comment `nil when latency is unavailable.` → `nil when packet transit is unavailable.` |
| `:72` | `func (l *Latency) Histogram() *stats.Histogram` → `func (l *PacketTransit) Histogram() *stats.Histogram` |
| `:92` | field `` Latency Latency `json:"latency"` `` → `` PacketTransit PacketTransit `json:"packetTransit"` `` (gofmt realigns `:91`) |
| `:115` | field `latency   Latency` → `transit   PacketTransit` |
| `:193` | `res.Latency = finishLatency(pr.latency)` → `res.PacketTransit = finishPacketTransit(pr.transit)` |
| `:222`, `:223`, `:327`, `:330`, `:333` | `pr.latency.` → `pr.transit.` |
| `:329` | `MaxPlausibleDelay` → `MaxPlausibleTransit` |
| `:334` | `live.Delay(v)` → `live.PacketTransit(v)` |
| `:342` | `func finishLatency(l Latency) Latency` → `func finishPacketTransit(l PacketTransit) PacketTransit` |

New and modified signatures, exactly:

```go
const MaxPlausibleTransit = 60 * time.Second
type PacketTransit struct { /* fields unchanged: Available, Reason, Negotiated, Stamped, Invalid, Ms, hist */ }
func (l *PacketTransit) Histogram() *stats.Histogram
func finishPacketTransit(l PacketTransit) PacketTransit
// in Result:   PacketTransit PacketTransit `json:"packetTransit"`
// in progress: transit PacketTransit
```

**Expected behaviour:** identical to today — same four reasons in the same order
(`:344-351`), same 1 % invalid-share rule (`:39`, `:348`), same samples. Only the names and
the JSON key change.

#### `internal/metrics/metrics.go`

| Line | Old → new |
|---|---|
| `:18` | comment `// DelayBucketsMs are the upper bounds of the one-way delay histogram.` → `// TransitBucketsMs are the upper bounds of the packet transit histogram.` |
| `:19` | `var DelayBucketsMs` → `var TransitBucketsMs` (values unchanged) |
| `:36` | `delayBuckets [12]atomic.Uint64 // len(DelayBucketsMs)` → `transitBuckets [12]atomic.Uint64 // len(TransitBucketsMs)` |
| `:37` | `delayCount` → `transitCount` |
| `:38` | `delaySumUs` → `transitSumUs` |
| `:50` | comment `// Delay records one one-way delay sample.` → `// PacketTransit records one packet transit sample, in milliseconds.` |
| `:51` | `func (l *Live) Delay(ms float64)` → `func (l *Live) PacketTransit(ms float64)` |
| `:55`, `:96` | `DelayBucketsMs` → `TransitBucketsMs` |
| `:57`, `:97` | `l.delayBuckets` → `l.transitBuckets` |
| `:61`, `:100` | `l.delayCount` → `l.transitCount` |
| `:62`, `:102` | `l.delaySumUs` → `l.transitSumUs` |
| `:93` | `const h = "whipbench_one_way_delay_seconds"` → `const h = "whipbench_packet_transit_seconds"` |
| `:94` | HELP text `Send-stamp to arrival delay of stamped packets (network plus server, not glass-to-glass).` → `Packet transit: arrival time minus the send-time stamp, per stamped packet (network plus server, not glass-to-glass).` |

Signature: `func (l *Live) PacketTransit(ms float64)` — behaviour unchanged (nil receiver,
negative and NaN ignored, `:52-54`).

#### `internal/report/report.go`

| Line | Old → new |
|---|---|
| `:104` | `` Latency Latency `json:"latency"` `` → `` PacketTransit PacketTransit `json:"packetTransit"` `` |
| `:107-108` | comment → `// PacketTransit is the pooled packet transit of every stamped packet of every` / `// viewer that had it.` |
| `:109` | `type Latency struct` → `type PacketTransit struct` (fields unchanged) |
| `:173` | `var noLatency []string` → `var noTransit []string` |
| `:199`, `:203` | `v.Latency.` → `v.PacketTransit.` |
| `:201`, `:218`, `:220`, `:222`, `:225`, `:227` | `a.Latency.` → `a.PacketTransit.` |
| `:203`, `:221`, `:222`, `:227` | `noLatency` → `noTransit` |
| `:290` | Method line, whole string → `"Packet transit: the publisher stamps each packet's wall-clock send time in the abs-capture-time RTP header extension; a viewer's sample is its arrival time minus the stamp. It is network plus server forwarding plus both clients' stacks, per packet rather than per frame, on one host or synchronised clocks — not glass-to-glass. When the server does not negotiate or forward the extension, packet transit is reported unavailable, never estimated."` |
| `:291` | in the Method line, `latency pools every valid sample of every viewer with latency,` → `packet transit pools every valid sample of every viewer that has it,` |

Signature: `type PacketTransit struct { Available bool; Reason string; Viewers int; Ms *stats.Summary }`
with the tags of `:110-113` unchanged; field `PacketTransit PacketTransit \`json:"packetTransit"\`` in `Aggregate`.
Do NOT touch `Schema` (`:30-31`).

#### `internal/report/markdown.go`

| Line | Old → new |
|---|---|
| `:57`, `:64`, `:65`, `:66`, `:67` | `a.Latency.` → `a.PacketTransit.` |
| `:58` | `row("one-way delay (stamped packets)", *a.Latency.Ms, " ms")` → `row("packet transit (stamped packets)", *a.PacketTransit.Ms, " ms")` |
| `:65` | format `"**Latency: unavailable** — %s.\n\n"` → `"**Packet transit: unavailable** — %s.\n\n"` |
| `:67` | format `"Latency %s.\n\n"` → `"Packet transit %s.\n\n"` |
| `:101` | in the viewers table header, the cells `latency p50` and `latency p99` → `transit p50` and `transit p99` (column count unchanged) |
| `:108`, `:110`, `:118` | variables `lat50, lat99` → `pt50, pt99` |
| `:109`, `:110` | `v.Latency.` → `v.PacketTransit.` |

#### `cmd/whipbench/main.go`

| Line | Old → new |
|---|---|
| `:271`, `:272`, `:274` | `a.Latency.` → `a.PacketTransit.` |
| `:272` | format `"one-way delay p50 %.1f ms, p99 %.1f ms\n"` → `"packet transit p50 %.1f ms, p99 %.1f ms\n"` |
| `:274` | format `"latency unavailable: %s\n"` → `"packet transit unavailable: %s\n"` |

#### `internal/publisher/publisher.go` and `internal/stats/stats.go`

| Line | Old → new |
|---|---|
| `publisher.go:8` | `its arrival time to get the one-way delay through the network and the server.` → `its arrival time to get the packet transit through the network and the server.` |
| `stats.go:3` | `histogram for the per-packet one-way delays, which are too many to keep.` → `histogram for the per-packet transit times, which are too many to keep.` |

#### `scripts/check-repo.sh`

| Line | Old → new |
|---|---|
| `:15` | message `README.md must say latency is not glass-to-glass` → `README.md must say packet transit is not glass-to-glass` (the grepped phrase `not glass-to-glass` stays) |
| `:16` | whole line → `grep -q "packet transit: unavailable" README.md \|\| fail "README.md must say a stripped extension gives 'packet transit: unavailable'"` |

#### `README.md` (phrases only — S4 rewrites the section around them)

Every edit is inside its line; no line is added or removed, so S4's line numbers hold.

| Line | Old → new |
|---|---|
| `:5` | `join time, one-way delay, packet loss,` → `join time, packet transit, packet loss,` |
| `:53` | the table row `\| one-way delay \| arrival time − the publisher's send-time stamp, per packet; see the next section \|` → `\| packet transit \| arrival time − the publisher's send-time stamp, per packet — not the one-way delay of a frame; see the next section \|` |
| `:56` | `one-way delay pools every packet of every viewer` → `packet transit pools every packet of every viewer` |
| `:68` | `**latency: unavailable**` → `**packet transit: unavailable**` |
| `:103` | `and a histogram of one-way delay.` → `and a histogram of packet transit (\`whipbench_packet_transit_seconds\`).` |
| `:107` | `and latency was unavailable because` → `and packet transit (the 0.0.1 reports' \`latency\` key) was unavailable because` |
| `:125` | `**Latency needs the extension**` → `**Packet transit needs the extension**` |

Leave `:7`, `:60` (the heading), `:62`, `:64`, `:66-67`, `:69-71` and `:146` to S4, and
`:113-114` (other tools) alone.

#### `CLAUDE.md` and `AGENTS.md`

| Line | Old → new |
|---|---|
| `CLAUDE.md:22` | `WHEP viewer: join times, stats, latency or why not` → `WHEP viewer: join times, stats, packet transit or why not` (keep the column alignment) |
| `CLAUDE.md:54-55` | rule 5, both lines → `5. **Packet transit and one-way delay are network plus server, not glass-to-glass**,` / `   and they need one clock or synchronised clocks. No metric is called "latency" (WB-1);` / `   one-way delay is WB-38 and capture-to-decode WB-2, neither implemented yet.` (three lines) |
| `AGENTS.md:5` | `hosts only, latency is not glass-to-glass, pure Go` → `hosts only, delay is not glass-to-glass, pure Go` |

#### `CHANGELOG.md`

**Where:** first bullet under `## [Unreleased]` → `### Changed` (insert after `:8`,
before the backlogsync bullet at `:9`). Exact text — the first line must carry both
`packet transit` and `WB-1`, because the Done-when greps one line:

```
- The 0.0.1 delay figure is renamed **packet transit** (WB-1) — per packet, arrival
  minus the abs-capture-time send stamp — so no metric is called "latency" and "one-way
  delay" is left for the per-frame figure of WB-38. Breaking: the report key `latency`
  is now `packetTransit` (the schema stays `whipbench.report/v0`, since no key changed
  meaning), the Prometheus histogram `whipbench_one_way_delay_seconds` is now
  `whipbench_packet_transit_seconds`, and the Markdown and stdout say "packet transit".
  A 0.0.1 parser or dashboard meets a missing key, not an error; there is no alias.
```

#### `thoughts/WB-1-latency-method/99-progress.md` (new, before any other edit)

Copy the qrspi plugin's template `skills/qrspi/references/99-progress.md` (in Claude Code
it is under `${CLAUDE_PLUGIN_ROOT}`; `find ~/.claude -path '*qrspi/references/99-progress.md' | head -1`
finds it), then:

1. H1 → `# 99 · Progress — WB-1 Latency method`.
2. Step status table: five rows `S1`…`S5`, each `⬜ todo | — | — |`; the Note of S5 is
   `human — the maintainer`.
3. "Where I left off": **Current step:** `S1`; "Done so far" `- nothing yet`; "Next
   concrete action" `- S1, 04-plan.md § S1`; "Modified but uncommitted files" `- none`.
4. Discoveries, Deviations, Verifications run, Context budget: keep the headings and
   table headers; delete the placeholder rows and the example Deviation block (its heading starts `### D`).
5. Append a last section:

```
---

## Observations

Human observations the plan's steps close on (S5). One line each.
```

Then set S1 to `🔄 in progress`. After S1's commit: S1 `✅ done` with the short SHA, and
one row per Verify command below in "Verifications run".

### Tests

| Case | Input | Expected |
|---|---|---|
| `TestPacketTransitUnavailableIsNeverANumber` (renamed from `TestLatencyUnavailableIsNeverANumber`, `report_test.go:88-101`) | as today: 3 viewers whose `PacketTransit` is `viewer.PacketTransit{Negotiated: false, Reason: "not negotiated: the WHEP answer did not accept abs-capture-time"}` | `r.Aggregate.PacketTransit.Available == false`, `.Ms == nil`; Markdown contains `**Packet transit: unavailable** — not negotiated` and does **not** contain `packet transit (stamped`; failure message `packet transit %+v` (`:95`) |
| `TestPacketTransitKeys` (new, `report_test.go`, after the one above) | `r := build(sc(3), viewers(3, 0))`; `b, err := r.JSON()` | `err == nil`; `strings.Count(string(b), "\"packetTransit\": {") == 4` (aggregate + 3 viewers); `strings.Contains(strings.ToLower(string(b)), "latency") == false`; `strings.Contains(string(b), "one-way delay") == false`. Doc comment: `// The 0.0.1 figure is packetTransit in the JSON and in the Method lines; no key, no definition is called latency (WB-1, D1).` Signature `func TestPacketTransitKeys(t *testing.T)` |
| `TestExposition` (`metrics_test.go:9-44`) | `l.PacketTransit(3)`, `(40)`, `(9000)`, `(-1)` instead of `l.Delay(…)` at `:15-18`; comment at `:18` → `// ignored: a negative transit is a clock problem, not a sample` | the eight wanted strings at `:28-35` with `whipbench_one_way_delay_seconds` → `whipbench_packet_transit_seconds`, values unchanged (`le="0.002"} 0`, `le="0.005"} 1`, `le="0.05"} 2`, `le="5"} 2`, `le="+Inf"} 3`, `_count 3`, `_sum 9.043`, `# TYPE … histogram`); **new** after the loop: `if strings.Contains(body, "one_way_delay") { t.Errorf("the 0.0.1 series must be gone:\n%s", body) }` |
| `TestNilLiveIsANoOp` (`metrics_test.go:46-50`) | `l.PacketTransit(1)` instead of `l.Delay(1)` at `:49` | no panic |
| `TestRoundTripVP8` (`runner_test.go:84-115`) | unchanged run | `:94`, `:95`, `:97`: `a.Latency` → `a.PacketTransit`; message at `:95` → `packet transit over loopback through the relay must be available: %+v`; same thresholds |
| `TestRoundTripH264` (`runner_test.go:117-128`) | unchanged | `:124`: `v.Latency.Available` → `v.PacketTransit.Available` |
| `TestStrippedExtensionIsUnavailableNotGuessed` (`runner_test.go:130-146`) | unchanged | `:137`, `:138`, `:140`, `:141`: `a.Latency` → `a.PacketTransit`; message at `:138` → `packet transit must be unavailable when the stamp is stripped: %+v`; `:143` wants `**Packet transit: unavailable**`; `:140` still wants `no stamps arrived` |

### Verify

```sh
gofmt -w cmd internal
go build ./... && go vet ./... && test -z "$(gofmt -l .)"; echo "exit $?"
go test -count=1 -run 'TestPacketTransit|TestExposition|TestNilLiveIsANoOp' ./internal/report/ ./internal/metrics/; echo "exit $?"
go test -race -count=1 -run 'TestRoundTrip|TestStrippedExtension' ./internal/runner/; echo "exit $?"
go test ./... && go vet ./... && test -z "$(gofmt -l .)" && ./scripts/check-repo.sh && node scripts/leakcheck.mjs; echo "exit $?"
golangci-lint run ./...; echo "exit $?"
grep -q 'packet transit: unavailable' README.md && grep -q 'not glass-to-glass' README.md; echo "exit $?"
grep -q 'whipbench_packet_transit_seconds' internal/metrics/metrics.go; echo "exit $?"
grep -q 'packetTransit' internal/viewer/viewer.go; echo "exit $?"
sed -n '/^## \[Unreleased\]/,/^## \[/p' CHANGELOG.md | grep -w 'WB-1' | grep -qi 'packet transit'; echo "exit $?"
! git grep -ni latency -- '*.go' scripts/check-repo.sh AGENTS.md ':!internal/report/report_test.go'; echo "exit $?"
! git grep -n -E 'one-way delay|one_way_delay|MaxPlausibleDelay|DelayBucketsMs|\.Delay\(|finishLatency' -- '*.go' ':!internal/report/report_test.go' ':!internal/metrics/metrics_test.go'; echo "exit $?"
! grep -n -E 'latency or why not|\*\*Latency is' CLAUDE.md; echo "exit $?"
```

Every line prints `exit 0`. (`golangci-lint` is the local binary; CI builds its own.)

### Acceptance criteria

- [ ] Every test case above passes; `go test -race -count=1 ./...` green
- [ ] Every Verify line prints `exit 0`
- [ ] `git diff --stat origin/main` lists only the files in "Files touched"
- [ ] One commit, no attribution trailer; `99-progress.md` updated

---

## S2 · Write the four new follow-up items, WB-38 to WB-41

Worktree `wb-1/s2-s3`, in parallel with S1. One commit: `WB-1 S2: WB-38 to WB-41, the
follow-up items`. Do not touch `99-progress.md`; report to the S1 owner.

**Ids first.** Run `grep -oE '\*\*WB-[0-9]+ — ' BACKLOG.md | grep -oE '[0-9]+' | grep -v '^99$' | sort -n | tail -1`.
It must print `37`. If it prints a higher number N, the four items take N+1…N+4 in the
order below, and every later `WB-38`…`WB-41` in this plan (S3, S4, and S1's CLAUDE.md and
CHANGELOG text) shifts by the same amount — record that as a Deviation in `99-progress.md`.

### Files touched

| Path | Action |
|---|---|
| `BACKLOG.md` | modify — four items inserted, nothing else changed |
| `ROADMAP.md` | regenerated with `npm run roadmap`, never hand-edited |

### Changes

#### `BACKLOG.md`

Each item is a bullet followed by indented (two-space) continuation lines, with no blank
line inside it; the meta comment ends the item. Insert the text exactly as below.

**WB-38 — where:** last item of `## v0.1.0 — The latency method, and a first report that
means something`, directly after the WB-8 item (it ends at `:76` with
`whatever the ramp. <!-- wb: prio=med size=S labels=measurement -->`) and before the blank
line that precedes `## v0.2.0`:

```
- [ ] **WB-38 — One-way delay by frame fingerprint**: the headline delay figure, and the
  one WB-5 publishes (WB-1, D2 and D4 in `thoughts/WB-1-latency-method/02-design.md`). In
  `run` the publisher logs t0 = `time.Now()` just before each frame's first `WriteRTP`,
  under a 64-bit hash of the frame's depacketised bytes: the whole VP8 frame, and for
  H.264 the VCL NAL units only, because a server may add or repeat SPS/PPS. Every viewer
  reassembles frames (pion `samplebuilder`), hashes them the same way and takes the
  latest send of that hash before t1, the arrival of the frame's marker packet. One
  sample per complete frame, NACK-recovered packets included; incomplete frames are
  counted, not sampled. Monotonic clock at both ends, report keys under `oneWayDelay`,
  source `fingerprint`; no header extension and no decoder, so it survives a server that
  drops abs-capture-time. The report states its limit: a frame later than the 4 s clip
  loop is ambiguous, and its sample is invalid. Byte-identical frames are excluded when
  the clip loads; a bitstream rewrite counts as `unmatchedFrames`; a server that drops
  the marker bit (WB-4) ends the frame at the last packet before the next RTP timestamp.
  Open: the CPU per viewer of reassembly and hashing at scale is unmeasured (WB-25's
  ceiling). <!-- wb: prio=high size=L labels=measurement,client -->
```

**WB-39, WB-40, WB-41 — where:** the last three items of `## v0.2.0 — Simulcast, layer
switches and metrics`, in this order, directly after the WB-19 item (it ends at `:93`
with `measured. <!-- wb: prio=low size=M labels=client -->`) and before the blank line
that precedes `## v0.3.0`:

```
- [ ] **WB-39 — abs-capture-time once per frame**: the stamp source beside WB-38's
  fingerprint (WB-1, D3). The publisher writes abs-capture-time = t0 on each frame's
  first packet only, instead of on every packet, and the viewer joins it to its frame by
  RTP timestamp. In `run` the publisher keeps the values it sent, so a stamp outside that
  set is counted as rewritten, never sampled. Each leg is classified — publisher
  `negotiated` or `dropped`, viewer `forwarded`, `rewritten`, `dropped` or `unverified`
  (split runs) — and the figure is a wall-clock difference, exposed to clock steps
  (WB-40). It ships under `oneWayDelay` with source `stamp` and retires packet transit,
  the 0.0.1 per-packet figure, with its `packetTransit` key and its Prometheus series.
  Open: whether pion's NACK responder resends the stored packet with its header
  extensions, so that a retransmitted first packet still carries the stamp.
  <!-- wb: prio=med size=M labels=measurement,client -->
- [ ] **WB-40 — Topology, clock and comparability in the report**: what makes two delay
  figures comparable (WB-1, D6 and D8). Every report gains `topology` (`single-process`
  or `split`) and `clock` (`method`, `offsetMs`, `uncertaintyMs`, `stepDetected`), and no
  host names. In `run` the method is `monotonic` for the fingerprint or `same-wall-clock`
  for the stamp, and wall-clock and monotonic elapsed time more than 0.1 ms apart at run
  end set `stepDetected`; in `view` the offset comes from WB-3's exchange, or the method
  is `none` and the figure `comparable: false`. Each source gets one block — source,
  available, reason, frames, samples, invalid, rewritten, unmatched, the summary in ms,
  uncertaintyMs, comparable, notComparableReason — and never a number when unavailable.
  Two reports rank only if both are comparable, share scenario, clip and source, and
  carry an uncertainty of 1 ms or less. `sourcesDisagree` flags a forwarded stamp whose
  p50 differs from the fingerprint's beyond the uncertainty; nothing is averaged, and
  there is no merged best source. <!-- wb: prio=high size=M labels=report,measurement -->
- [ ] **WB-41 — Sample window and retransmission beside delay**: what one-way delay is
  sampled over, and what sits next to it (WB-1, D7). A new scenario key
  `excludeFirstSeconds` (default 5) drops each viewer's first seconds, counted from its
  own first RTP packet; `warmupSeconds` keeps its meaning. Beside one-way delay go the
  NACKs each viewer sent (pion's stats interceptor, registered with the default
  interceptors in `internal/rtc/rtc.go` and never read) and `lateCompletedFrames`,
  frames whose gap was filled after their marker arrived; RTX stays unnegotiated. The
  1% histogram stays; the Method line says "±0.5 % of value" and values print to 0.1 ms.
  Open: whether the stats interceptor's NACK count means NACKs sent, and how recovered
  packets relate to `tooLate` in `internal/rtpstats/rtpstats.go`.
  <!-- wb: prio=med size=M labels=measurement,report -->
```

Leave every other line of `BACKLOG.md` as it is — WB-1, WB-2, WB-3, WB-4 and WB-5 are
S3's.

#### `ROADMAP.md`

`npm run roadmap` after the edit; commit the result with `BACKLOG.md`.

### Tests

| Case | Input | Expected |
|---|---|---|
| four new ids | `grep -cE '\*\*WB-(38\|39\|40\|41) — ' BACKLOG.md` | `4` |
| WB-38 placed in v0.1.0 | `awk '/^## /{h=$0} index($0,"**WB-38 — "){print h}' BACKLOG.md` | starts `## v0.1.0` |
| WB-39..41 placed in v0.2.0 | same awk with `WB-39`, `WB-40`, `WB-41` | each starts `## v0.2.0` |
| WB-38 carries D4, VCL NAL, the 4 s limit, the CPU fact | `item WB-38` | contains `D4`, `VCL NAL`, `4 s`, `CPU per viewer` |
| WB-39 carries D3, retires packet transit, the NACK fact | `item WB-39` | contains `D3`, `packet transit`, `NACK` |
| WB-40 cites D6 and D8 | `item WB-40` | contains `D6`, `D8` |
| WB-41 cites D7, the key, the NACK fact | `item WB-41` | contains `D7`, `excludeFirstSeconds`, `NACK` |
| parser accepts them | `npm run backlog` | `ok — 41 items, 8 milestones; ROADMAP.md is in step with BACKLOG.md` |

### Verify

```sh
item() { awk -v id="**$1 — " 'index($0,id){p=1;print;next} /^- \[|^## /{p=0} p' BACKLOG.md; }
npm run roadmap && git diff --exit-code ROADMAP.md; echo "exit $?"   # after the commit
npm run backlog && node scripts/leakcheck.mjs; echo "exit $?"
test "$(grep -cE '\*\*WB-(38|39|40|41) — ' BACKLOG.md)" = 4; echo "exit $?"
for n in 38 39 40 41; do awk -v id="**WB-$n — " '/^## /{h=$0} index($0,id){print h}' BACKLOG.md; done
item WB-38 | grep -q 'D4' && item WB-38 | grep -q 'VCL NAL' && item WB-38 | grep -q '4 s' && item WB-38 | grep -q 'CPU per viewer'; echo "exit $?"
item WB-39 | grep -q 'D3' && item WB-39 | grep -qi 'packet transit' && item WB-39 | grep -q 'NACK'; echo "exit $?"
item WB-40 | grep -q 'D6' && item WB-40 | grep -q 'D8'; echo "exit $?"
item WB-41 | grep -q 'D7' && item WB-41 | grep -q 'excludeFirstSeconds' && item WB-41 | grep -q 'NACK'; echo "exit $?"
git diff --stat origin/main   # BACKLOG.md and ROADMAP.md only
```

The `for` line prints `## v0.1.0 …` once and `## v0.2.0 …` three times; every other line
prints `exit 0`.

### Acceptance criteria

- [ ] Every test case above holds
- [ ] `npm run backlog` reports 41 items
- [ ] Only `BACKLOG.md` and `ROADMAP.md` changed; one commit, no attribution trailer

---

## S3 · Rewrite WB-2, WB-3 and WB-4

Worktree `wb-1/s2-s3`, after S2's commit. One commit: `WB-1 S3: WB-2, WB-3 and WB-4
rewritten; the citing lines follow`. Do not touch `99-progress.md`; report to the S1
owner. S2 shifted line numbers, so every edit below is anchored on text; the `fe24f42`
line is given for orientation only.

### Files touched

| Path | Action |
|---|---|
| `BACKLOG.md` | modify — three items rewritten, WB-2 moved, four citing passages edited |
| `ROADMAP.md` | regenerated with `npm run roadmap` |

Every line in `BACKLOG.md` that cites WB-2, WB-3, glass-to-glass or a "latency" figure,
and what happens to it (listed by `grep -n -E 'WB-2\b|WB-3\b|glass|latency' BACKLOG.md`
at `fe24f42`):

| `fe24f42` line | Text | Action |
|---|---|---|
| `:25` | the v0.1.0 milestone title | **stays** (maintainer, Structure approval) |
| `:27-31` | v0.1.0 intro: "0.0.1 measures one-way delay …", "latency — through the network, and glass to glass —" | **rewrite** (1 below) |
| `:33-39` | WB-1 — its open questions, incl. "(WB-2)" and "what 'latency' means" | **stays** — WB-1's own record; closing it is the maintainer's, after S5 |
| `:40-46` | WB-2 | **rewrite and move** (2 below) |
| `:47-51` | WB-3 | **rewrite in place** (3 below) |
| `:52-56` | WB-4 | **rewrite in place** (4 below) |
| `:57-59` | WB-5: "with WB-1's method", "carries a latency figure" | **rewrite body, title stays** (5 below) |
| `:97-100` | WB-20: "WB-3's clock discipline" | **edit** (6 below) |
| `:116-120` | WB-24: "Needs WB-3's clock discipline", "no latency samples" | **edit** (6 below) |
| `:131` | v0.5.0 intro "A server's latency regression" | stays — the subject, not a metric |
| `:148` | v0.6.0 intro "Once viewers decode (WB-2)" | stays — true of the rewritten WB-2 |
| `:189-190`, `:192-213` | v0.0.1 intro and the shipped items WB-9..WB-14 ("one-way delay" at `:198`) | stay — history |

### Changes

#### `BACKLOG.md`

**1. v0.1.0 intro.** Replace the paragraph that starts `0.0.1 measures one-way delay only
where` and ends `` `evals/`.** `` (five lines, `fe24f42:27-31`) with:

```
0.0.1 measures packet transit only where the server forwards abs-capture-time, and the
first server tried does not negotiate it. This milestone decides how whipbench measures
delay — WB-1 chose one-way delay per frame, by frame fingerprint (WB-38) — runs it
against MediaMTX, and publishes that report. **No v0.1.0 before WB-1 has a decided
method and WB-5 is in `evals/`.**
```

**2. WB-2.** Delete the WB-2 item (from `- [ ] **WB-2 — Visual timestamp for
glass-to-glass**` through its meta line `<!-- wb: prio=high size=L labels=research,measurement -->`,
`fe24f42:40-46`). Insert this as the **first** item of `## v0.6.0 — What the viewer
actually sees`: after the intro paragraph (it ends `server copes.`) and its blank line,
before `- [ ] **WB-30 — Freezes and frame drops**`:

```
- [ ] **WB-2 — Keyframe capture-to-decode for VP8**: the third delay figure (WB-1, D5),
  built here beside transcoding (WB-31), where it first measures something WB-38 cannot.
  `scripts/make-clips.sh` draws a block code — a 16-bit frame index plus check bits,
  sized for 600 kbit/s — into each source frame, and the clips are regenerated once. Up
  to 10 sampled viewers decode keyframes only, with `golang.org/x/image/vp8` (BSD-3, pure
  Go, so the pure-Go rule holds), read the index back and take the send time from
  WB-38's send log, keyed by index: capture-to-decode is the keyframe's decode at the
  endpoint decoder's output minus t0. Unreadable codes are counted, never guessed;
  keyframes are the largest frames, so the figure is biased high and labelled so; no
  H.264. `methodsDisagree` flags it against WB-38's figure, never averaged. A vetted
  pure-Go inter-frame decoder would upgrade it to every frame. Open: whether a code
  drawn by `make-clips.sh` survives libvpx at 600 kbit/s and reads back after decode.
  <!-- wb: prio=med size=L labels=measurement,research -->
```

**3. WB-3.** Replace the WB-3 item (from `- [ ] **WB-3 — Clock synchronisation between
machines**` through `<!-- wb: prio=high size=M labels=research,measurement -->`,
`fe24f42:47-51`) in place with:

```
- [ ] **WB-3 — Clock exchange between publish and view**: how a split run, publisher and
  viewers on different hosts, measures the clock offset it runs with (WB-1, D6).
  `whipbench publish` answers a small clock responder, opt-in by flag, and
  `whipbench view --clock-peer HOST:PORT` runs an RTT-halving exchange against it at
  start, at end and every 30 s: offset ± min-RTT/2, piecewise-linear between the
  points, written into WB-40's `clock` block. Without `--clock-peer` the method is
  `none`, the delay `comparable: false`, "publisher clock not measured". The responder
  opens a second port between the load hosts; reports still carry no host names. RTCP
  sender reports and `chronyc` were rejected: SFUs originate their own SRs, and chrony
  reports each daemon's view of its upstream, not of the peer.
  <!-- wb: prio=high size=M labels=client,measurement -->
```

**4. WB-4.** Replace the WB-4 item (from `- [ ] **WB-4 — Header extensions, server by
server**` through `<!-- wb: prio=high size=M labels=research,benchmark -->`,
`fe24f42:52-56`) in place with:

```
- [ ] **WB-4 — What each server forwards: extensions, payload bytes, marker bit**: for
  MediaMTX, OvenMediaEngine, LiveKit and Janus, record whether the WHIP and WHEP answers
  negotiate abs-capture-time, whether the server forwards it, rewrites it or strips it,
  and which extensions it does forward (WB-39 rests on it); whether it forwards each
  depacketised VP8 frame, and each H.264 frame's VCL NAL units, byte for byte (WB-38's
  fingerprint rests on it); and whether it keeps the marker bit on each frame's last
  packet (WB-38's frame end). MediaMTX v1.21.1 negotiates abs-capture-time on neither
  leg (2026-10-01), and forwards frames byte for byte with one marker per frame on both
  codecs (2026-10-02, `evals/2026-10-02-mediamtx-fingerprint.md`); the other three are
  unverified. <!-- wb: prio=high size=M labels=research,benchmark -->
```

**5. WB-5.** Keep its first line's title `**WB-5 — Live run against MediaMTX with the
decided method**`; replace the whole item (`fe24f42:57-59`) with:

```
- [ ] **WB-5 — Live run against MediaMTX with the decided method**: the 0.0.1 smoke run
  repeated with WB-1's method, one-way delay by frame fingerprint (WB-38), published in
  `evals/` as the first report that carries a one-way delay figure or says, with
  evidence, why it cannot. <!-- wb: prio=high size=M labels=benchmark -->
```

**6. WB-20 and WB-24.** In WB-20, `and then on separate machines with WB-3's` /
`clock discipline)` → `and then on separate machines with WB-3's` / `clock exchange)` (the
word `discipline` → `exchange`, `fe24f42:98-99`). In WB-24, replace the two lines
`merges their reports into one, per host and in total. Needs WB-3's clock discipline;` /
`an agent whose clock offset is unknown contributes no latency samples.`
(`fe24f42:118-119`) with:

```
  merges their reports into one, per host and in total. Needs WB-3's clock exchange;
  an agent whose clock offset is unknown contributes no comparable one-way delay (WB-40).
```

The WB-2, WB-3 and WB-4 retitles rename their GitHub issues on merge, and WB-2's issue
moves to the v0.6.0 milestone — intended (Minimum context, the issue sync).

#### `ROADMAP.md`

`npm run roadmap`; commit with `BACKLOG.md`.

### Tests

| Case | Input | Expected |
|---|---|---|
| WB-2 moved | `awk '/^## /{h=$0} index($0,"**WB-2 — "){print h}' BACKLOG.md` | `## v0.6.0 — What the viewer actually sees <!-- ms: phase=later -->` |
| WB-2 content | `item WB-2` | contains `capture-to-decode`, `libvpx`, `D5` |
| old WB-2 title gone | `grep -c 'Visual timestamp for glass-to-glass' BACKLOG.md` | `0` |
| WB-3 content | `item WB-3` | contains `--clock-peer`, `publish`, `D6` |
| WB-4 content | `item WB-4` | contains `marker` and `byte` (case-insensitive) |
| citing lines | `grep -n -E 'glass to glass\|latency samples\|latency figure\|clock discipline' BACKLOG.md` | no output |
| milestone title kept | `grep -c '^## v0.1.0 — The latency method, and a first report that means something <!-- ms: phase=now -->$' BACKLOG.md` | `1` |
| WB-3, WB-4, WB-5 stay in v0.1.0 | the WB-2 awk with `WB-3`, `WB-4`, `WB-5` | each `## v0.1.0 …` |
| parser | `npm run backlog` | `ok — 41 items, 8 milestones; …` |

### Verify

```sh
item() { awk -v id="**$1 — " 'index($0,id){p=1;print;next} /^- \[|^## /{p=0} p' BACKLOG.md; }
npm run roadmap && git diff --exit-code ROADMAP.md; echo "exit $?"   # after the commit
npm run backlog && node scripts/leakcheck.mjs; echo "exit $?"
awk '/^## /{h=$0} index($0,"**WB-2 — "){print h}' BACKLOG.md | grep -q 'v0.6.0'; echo "exit $?"
item WB-2 | grep -q 'capture-to-decode' && item WB-2 | grep -q 'libvpx'; echo "exit $?"
! grep -q 'Visual timestamp for glass-to-glass' BACKLOG.md; echo "exit $?"
item WB-3 | grep -q -- '--clock-peer' && item WB-3 | grep -q 'publish'; echo "exit $?"
item WB-4 | grep -qi 'marker' && item WB-4 | grep -qi 'byte'; echo "exit $?"
! grep -n -E 'glass to glass|latency samples|latency figure|clock discipline' BACKLOG.md; echo "exit $?"
for n in 3 4 5 38; do awk -v id="**WB-$n — " '/^## /{h=$0} index($0,id){print h}' BACKLOG.md; done
git diff --stat HEAD~1   # BACKLOG.md and ROADMAP.md only
```

Every `exit` line prints `exit 0`; the `for` line prints `## v0.1.0 …` four times.

### Acceptance criteria

- [ ] Every test case above holds
- [ ] S2's checks still pass (re-run S2's Verify block)
- [ ] Only `BACKLOG.md` and `ROADMAP.md` changed; one commit, no attribution trailer

---

## S4 · Point the README at the decision

Branch `wb-1/s1`, after S1 is committed there and `wb-1/s2-s3` (S2 + S3) is merged into
it: `git -C "$WT/wb1-s1" merge --no-ff wb-1/s2-s3 -m "WB-1: merge the backlog steps"`. One
commit: `WB-1 S4: the README says what WB-1 decided`. The owner updates `99-progress.md`
(S2, S3 rows from the S2/S3 session's report; S4).

### Files touched

| Path | Action |
|---|---|
| `README.md` | modify — `:7`, the section `:60-71`, `:146` |
| `thoughts/WB-1-latency-method/99-progress.md` | modify — status rows |

S1 edited README lines in place, so the `fe24f42` line numbers below still hold.
Keep, verbatim, S1's phrases and everything `scripts/check-repo.sh` greps
(`not glass-to-glass` at `:66`, `packet transit: unavailable` at `:68`).

### Changes

#### `README.md`

**`:7`** — replace the whole line with:

```
**Status: 0.0.1, not released.** The clients, the report and the in-process test relay work, and the first live run against MediaMTX is in [`evals/`](evals/). MediaMTX does not negotiate the header extension 0.0.1's stamp travels in, so against it packet transit is reported **unavailable** — by design, never estimated. The delay method of the next milestone is decided ([WB-1](BACKLOG.md)): one-way delay per frame, by frame fingerprint, which needs no header extension ([WB-38](BACKLOG.md)).
```

**The section `## Latency, and its limits` (`:60-71`).** The heading stays (maintainer).

1. After the heading's blank line (`:61`), insert this paragraph and a blank line, before
   `:62`:

   ```
   No figure whipbench reports is called "latency": the word covers too many different numbers to rank two servers by. [WB-1](BACKLOG.md) decided which ones whipbench measures; this section says what each is and what it is not.
   ```

2. `:62` — prefix the line with `**Packet transit — what 0.0.1 reports.** ` (bold lead,
   one space), the rest of the line unchanged (it starts `The publisher writes the
   wall-clock time each RTP packet`).
3. `:64`, `:66`, `:68`, `:69` — unchanged.
4. `:67` — replace its last sentence `How to bound it is [WB-3](BACKLOG.md).` with
   `Measuring that error between two hosts is [WB-3](BACKLOG.md)'s clock exchange.`
5. `:71` — replace the whole line (`Measuring glass-to-glass through a timestamp drawn
   into the frames is the open design question …`) with these two paragraphs, separated
   by one blank line:

   ```
   **One-way delay — the headline, decided by [WB-1](BACKLOG.md) and not built yet.** Per frame: from the publisher writing the frame's first packet to the viewer receiving its last, one sample per complete frame. It is still network plus server forwarding, still not glass-to-glass, and it is the figure servers are ranked by. Its send instant has two sources, side by side and never averaged: the **fingerprint** — each viewer hashes the frame it reassembled and looks up when the publisher sent those bytes, which needs no header extension and so works through a server like MediaMTX ([WB-38](BACKLOG.md), the figure v0.1.0's report is built on) — and the abs-capture-time **stamp**, once per frame instead of per packet, which replaces packet transit ([WB-39](BACKLOG.md)). Every figure will carry its topology, clock method, uncertainty and whether it can be compared with another report ([WB-40](BACKLOG.md)), and its sample window and retransmissions ([WB-41](BACKLOG.md)); between two hosts the clock offset comes from [WB-3](BACKLOG.md).

   **Capture-to-decode — later, in v0.6.0.** A frame index drawn into the clips and read back from decoded VP8 keyframes on a sample of viewers ([WB-2](BACKLOG.md)). It differs from one-way delay only where a server transcodes. A timestamp drawn live into the frames would need a live encoder, which whipbench does not have.
   ```

The next heading, `## Scenarios`, is untouched.

**`:146`** — two substitutions inside the line:
- `v0.1.0 decides the latency method (WB-1) and records a live run against MediaMTX with the first report; v0.2.0 adds simulcast, layer switches and metrics;`
  → `v0.1.0 measures one-way delay by frame fingerprint (WB-1 decided it, WB-38 builds it) and records a live run against MediaMTX with the first report; v0.2.0 adds simulcast, layer switches and metrics, and the stamp, clock and comparability around one-way delay (WB-39 to WB-41);`
- `v0.6.0 measures what the viewer sees — freezes,` → `v0.6.0 measures what the viewer sees — capture-to-decode (WB-2), freezes,`

**Do NOT touch:** `:5`, `:53`, `:56`, `:103`, `:107`, `:125` (S1's), `:113-114` (other
tools), `:122` (the Limits bullet "Not glass-to-glass").

### Tests

| Case | Input | Expected |
|---|---|---|
| section cites the new item | `sed -n '/^## Latency, and its limits/,/^## [^L]/p' README.md` | contains `WB-38` and `one-way delay` |
| section keeps S1's phrases | same range | contains `packet transit: unavailable` and `not glass-to-glass` |
| the open-question sentence is gone | `grep -c 'open design question of the next milestone' README.md` | `0` |
| every README backlog id exists | `grep -oE 'WB-[0-9]+' README.md \| sort -u` against `grep -oE '\*\*WB-[0-9]+ — ' BACKLOG.md` | every README id is a backlog item |
| repo invariants | `./scripts/check-repo.sh` | `ok — repo invariants hold at 0.0.1` |
| site builds | `npm run build:site` | exit 0 |

### Verify

```sh
npm ci --no-audit --no-fund   # the site build needs the devDependency marked
./scripts/check-repo.sh && node scripts/leakcheck.mjs && npm run build:site; echo "exit $?"
sed -n '/^## Latency, and its limits/,/^## [^L]/p' README.md | grep -q 'WB-38'; echo "exit $?"
sed -n '/^## Latency, and its limits/,/^## [^L]/p' README.md | grep -q 'one-way delay'; echo "exit $?"
sed -n '/^## Latency, and its limits/,/^## [^L]/p' README.md | grep -q 'packet transit: unavailable'; echo "exit $?"
! grep -q 'open design question of the next milestone' README.md; echo "exit $?"
for id in $(grep -oE 'WB-[0-9]+' README.md | sort -u); do grep -q "\*\*$id — " BACKLOG.md || echo "missing $id"; done
go test ./... && npm run backlog; echo "exit $?"
```

Every `exit` line prints `exit 0`; the `for` line prints nothing.

### Acceptance criteria

- [ ] Every test case above holds
- [ ] S1's Verify block still passes on the merged branch
- [ ] One commit for S4, no attribution trailer; `99-progress.md` rows S1-S4 `✅ done`

### Landing (after S4)

```sh
git -C "$WT/wb1-s1" log --format='%ae %s%n%b' origin/main..HEAD   # one public author address, no trailer
(cd "$WT/wb1-s1" && node scripts/leakcheck.mjs); echo "exit $?"   # home paths, e-mails, trailers: files and history
git -C "$WT/wb1-s1" push -u origin wb-1/s1
gh pr create --repo Allan-Nava/whipbench --base main --head wb-1/s1 \
  --title "WB-1: packet transit, the follow-up items WB-38 to WB-41, the README" \
  --body-file "$WT/pr-body.md"
```

`pr-body.md`: what S1-S4 changed (one line each), the breaking rename from the CHANGELOG
line, the four issues the merge will create and the three it retitles, and "S5 is the
maintainer's read; see `thoughts/WB-1-latency-method/04-plan.md` § S5". No tool
attribution footer. Wait for `gh pr checks --watch` to pass; the PR merges only after S5.

---

## S5 · The maintainer reads the result

**Who:** human — Allan Nava. An agent does not perform or tick this step.

**What is read, on the PR:** the README section "Latency, and its limits" and `:7`,
`:146`; the four new items WB-38 to WB-41; the rewritten WB-2, WB-3, WB-4 (and the edited
WB-5, WB-20, WB-24 and v0.1.0 intro); the CHANGELOG line. The question is whether they
say what `02-design.md` decided.

**What is recorded:** one line, appended under `## Observations` in
`thoughts/WB-1-latency-method/99-progress.md` (the section S1 created), committed to
`wb-1/s1` before the merge, in exactly this format:

```
- S5 observed: YYYY-MM-DD, name: what was read and whether it says what the Design decided
```

For example: `- S5 observed: 2026-10-03, Allan Nava: README "Latency, and its limits",
WB-38 to WB-41, WB-2/WB-3/WB-4 read in the PR — they say what the Design decided`. If
something does not, the line says what, and the fix goes back to the step that wrote it
before merging. The owner then sets S5 `✅ done` in the step table.

### Verify

```sh
grep -nE '^- S5 observed: [0-9]{4}-[0-9]{2}-[0-9]{2}, ' thoughts/WB-1-latency-method/99-progress.md
```

Prints exactly one line.

### Acceptance criteria

- [ ] The line exists, in the format above, written by the maintainer
- [ ] The PR is merged after it (`gh pr merge --squash`)

---

## Rollback

There is no production deployment; what goes out is a source tree, a public backlog that
becomes GitHub issues, and (later) a tagged binary.

- **Before merge:** close the PR and delete `wb-1/s1` and `wb-1/s2-s3`; nothing reached
  `main` or the issues.
- **After merge, everything:** `git revert` the squash commit on a branch and merge that
  PR. The rename is reversible until D2 ships under the freed name (D1). The issue sync
  then retitles WB-2, WB-3 and WB-4 back and moves WB-2 back to v0.1.0, but it **does
  not delete** the issues created for WB-38 to WB-41: close each by hand,
  `gh issue list --repo Allan-Nava/whipbench --search 'WB-38 in:title'` (and 39, 40, 41)
  then `gh issue close "$n" --reason "not planned"`.
- **After merge, the rename only** (a 0.0.1 consumer must keep `latency`): revert S1's
  changes **and** S4's README edits together — S4 builds on S1's phrases, and
  `scripts/check-repo.sh` must match the README in the same commit. The backlog (S2,
  S3) can stay.
- **After merge, the backlog only:** revert S2 and S3 together with S4 — S4's README
  cites WB-38 to WB-41 and the rewritten WB-2/WB-3 — then run `npm run roadmap` and
  commit `ROADMAP.md` in the same commit, and close the four issues as above.
- Nothing here touches the report schema or `evals/`, so no published report is
  invalidated by either direction.

---

## Status

- [x] Every step has exact paths
- [x] Every new function has a complete signature
- [x] Every test case has inputs and expected outputs
- [x] Every verification command is copy-pasteable
- [x] **Zero-context test:** an agent reading only this file can execute it
- [x] Rollback plan present
- [x] Approved (2026-10-02, Allan Nava, in chat: "ok procedi") — the identifier renames in S1 (`Live.Delay` → `PacketTransit` and the like) included

> Next phase: **Implement**. It receives: this file + `99-progress.md`.
> One session per step.
