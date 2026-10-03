# 04 · Plan — WB-38 One-way delay by frame fingerprint

**Written against:** `74a7ad2`

---

## References

- Structure: [`03-structure.md`](./03-structure.md) — S1-S8 agent steps, H1 the maintainer's
- Design: [`02-design.md`](./02-design.md) — D1-D8, with the accepted departures D4, D6, D3

---

## Minimum context for the executor

Everything the executor needs so it never has to explore. Line numbers are at `74a7ad2`;
where an earlier step shifts them, the step anchors on text as well.

| Fact | Where |
|---|---|
| Module `github.com/Allan-Nava/whipbench`, Go 1.27; direct deps include pion/webrtc v4.2.22 and pion/rtp v1.10.5. **No new dependency** (SHA-256 is `crypto/sha256`) | `go.mod:1-10`; `CLAUDE.md:123-125` |
| CI: `gofmt -l .` empty, `go vet ./...`, `go test -race -count=1 ./...`, static builds, `./scripts/check-repo.sh`; lint is golangci-lint v2.12.2 `default: standard` with `staticcheck` and `unused` disabled | `.github/workflows/ci.yml:27-39`, `:54-58`; `.golangci.yml` |
| `clip.Clip{Codec string; Ticks uint32; Frames []Frame}`, `clip.Frame{Data []byte; Key bool}`; `clip.VP8 = "vp8"`, `clip.H264 = "h264"`; `(*Clip).Frame(k uint64)`, `(*Clip).Timestamp(base uint32, k uint64) uint32` = base + k·Ticks with uint32 wrap; `clip.SplitAnnexB(b) [][]byte` strips start codes; `clip.Load(codec, data)` | `internal/clip/clip.go:22-65`, `:199-235` |
| H.264 `Frame.Data` is Annex-B **with** start codes, SPS (7) and PPS (8) before every IDR, one SEI (6) before the first; the embedded clip's NAL types are 1 (116), 5 (4), 6 (1), 7 (4), 8 (4) | `internal/clip/clip.go:159-197`; probe 2026-10-03 |
| Embedded clips `whipbench.ClipVP8`, `whipbench.ClipH264`: 120 frames each, `Ticks` 3000, 120 distinct fingerprints each. Fingerprint of frame 0: VP8 `0xcc7dc033284a903a`, H.264 `0x5000846a47ceb02b`; frame 1: VP8 `0x624f3ffa2b4897e8`, H.264 `0x25780bb4ae13506e`. First 64 bits of SHA-256("abc") = `0xba7816bf8f01cfea` | `clips.go:10-19`; probe 2026-10-03 on `74a7ad2` |
| Publisher: `MTU = 1200`; payloaders `&codecs.VP8Payloader{EnablePictureID: true}` and `&codecs.H264Payloader{}`, one per `Stream`; frame loop `for ; ; k++` (`k` is a `uint64`, `:225`) with the packet loop at `:247-265`, `WriteRTP` at `:257`; k advances by one per frame, a slip moves only `origin` (`:229-233`) | `internal/publisher/publisher.go:30`, `:32-45`, `:202-275` |
| Offline round trip (payloader at MTU 1200 → fresh depacketiser per frame → VCL hash): H.264 240/240, 8 STAP-A packets per 240 frames, up to 9 packets per frame; VP8 240/240, up to 13 packets per frame | probe 2026-10-03 |
| Viewer: `Config` `:41-52`; `PacketTransit` per-viewer type with unexported `hist` and `Histogram()` `:54-77`; `Result` `:79-103` (`PacketTransit` field `:97`); `progress` `:105-123`; the deferred finaliser fills `res` under `pr.mu` after the read loop ends `:169-199`; `OnTrack` `:217-233`; `readLoop` `:302-345`, whose local `codec` (`:304`) is `rtc.CodecName` of the track (`:224`): `"vp8"` or `"h264"`, the same strings as `clip.VP8`/`clip.H264`; arrival `now := time.Now()` `:312`, the lock held `:316-343` | `internal/viewer/viewer.go` |
| Report imports viewer (`report.go:27`), so any per-viewer type a report reads lives in package `viewer`, as `PacketTransit` does | `internal/report/report.go:24-29` |
| Report: `Schema` `:32` (stays v0); `Aggregate` `:80-106`; pooled `PacketTransit` `:110-115`; `Input` `:117-129`; `Build` `:132-171` (viewers sorted `:159`, `aggregate` called `:165`); `aggregate` `:173-236` (pooling pattern `:203-208`, reasons `:220-232`); `mostCommon` `:256-269`; `Method` `:289-297` (packet transit line `:294`) | `internal/report/report.go` |
| Markdown: aggregate rows `:58-66`, unavailable lines `:67-72`, viewers header `:105-106`, viewer row `:107-123` | `internal/report/markdown.go` |
| `stats.Histogram`: `NewHistogram()`, `Add(ms)`, `Count() uint64`, `Merge(o)`, `Summary() stats.Summary`; not safe for concurrent use | `internal/stats/stats.go:55-157` |
| Runner: clip check `:52-54`; `live` `:66`; `in := report.Input{…}` `:75`; publisher `Connect` `:82-84`; early return on publisher failure `:85-91`; `viewer.Run` `:132-135`; `report.Build` `:152`. Viewers that never started are built at `:129` without `viewer.Run` | `internal/runner/runner.go` |
| stdout headline `:302-311`; `run` loads a clip only when the scenario has a WHIP endpoint `:276-282`; `view` passes no clip `:246` | `cmd/whipbench/main.go` |
| Test relay: `Options` `:31-41`; `forward` builds a fresh header per leg and copies `Marker` `:215-217`; `DropEvery` drops at `:210-212`, before `WriteRTP`, so the relay's NACK responder never saw those packets | `internal/testserver/testserver.go` |
| **Join needs a marker**: a keyframe is complete only when its marker packet arrives | `internal/rtpstats/rtpstats.go:212-252` |
| pion depacketisers: `codecs.VP8Packet`, `codecs.H264Packet` (`IsAVC` false gives Annex-B with 4-byte start codes); both satisfy `rtp.Depacketizer`. `H264Packet.Unmarshal` errors on NAL types 0, 25, 26, 27, 29, 30, 31 (STAP-B, MTAP16/24, FU-B…) and keeps FU-A bytes across calls | `$(go env GOMODCACHE)/github.com/pion/rtp@v1.10.5/codecs/h264_packet.go:214-313`; `…/rtp@v1.10.5/depacketizer.go:7-19` |
| Test files: `internal/runner/runner_test.go` (package `runner_test`; `var loop = rtc.Options{LoopbackOnly: true}` `:32`; helper `run(t, opt, sc, bearer)` `:34-60`, `base(codec, n)` `:80-82` = ramp 1 s, hold 3 s, warmup 0.5 s, join timeout 5 s; `TestRoundTripVP8` `:84-122`, `TestRoundTripH264` `:147-158`, `TestLossThroughALossyRelay` `:194-205`); `internal/report/report_test.go` (package `report`; helpers `f`, `viewers(n, failed)`, `build(sc, vs)`, `sc(n)` `:15-41`; `TestPacketTransitKeys` `:104-121`); `internal/metrics/metrics_test.go:41-43`; `cmd/whipbench/main_test.go` (package `main`) | as listed |
| Backlog item shape, labels, highest id **WB-41** (`WB-99` at `BACKLOG.md:15` is the fenced format example); `npm run backlog` fails when `ROADMAP.md` is stale, `npm run roadmap` regenerates it | `BACKLOG.md:10-23`, `:96-111` (WB-38), `:154-164` (WB-41); `package.json` `scripts` |
| CLAUDE.md rule 1: a measurement's definition is said the same way in the README table, the report `Method` line and the package comment | `CLAUDE.md:44-45` |

**Conventions to respect:** English, British-leaning spelling, em-dashes, no marketing
filler, no decorative emoji (`CLAUDE.md:121-122`). Go: every exported identifier has a doc
comment that says *why* as well as what (as in `internal/viewer/viewer.go:32-39`); `gofmt
-w` decides alignment; integer conversions that cannot overflow carry
`//nolint:gosec // <reason>` like `internal/clip/clip.go:64`. Errors are returned, never
logged inside a library package. Nothing in a report's JSON — key, reason, constant — may contain the word `latency`, and
nothing in it but the one-way delay `Method` line may contain "one-way delay" in any case
(D8). Markdown and stdout text is not JSON and may say "one-way delay".
Commits carry **no** tool-attribution trailer or footer, and the author e-mail is the
repository's configured public one (`git config user.email`). Every command runs from the
repository root of the step's worktree.

**Shared gate.** A step is done only when its own Verify block **and** this exit 0:

```sh
test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./...; echo "exit $?"
golangci-lint run --disable=staticcheck --disable=unused ./...; echo "exit $?"
```

Print every exit code and read the whole output — never judge a run through `| tail`.

**Branches.** S1 ‖ S2 ‖ S3 ‖ S5 ‖ H1, then S4 ‖ S6, then S7, then S8 (`03-structure.md`,
Recommended execution order). One branch per step from `origin/main`, named
`wb-38/s<n>`, one pull request per step titled `WB-38 S<n>: <step title>`. S1 and S2 both
create `internal/fingerprint/`: land S1 first if both are ready; S2 rebases and re-runs the
gate. A step that depends on others starts from an `origin/main` that has them.

**`99-progress.md` — the first action of the first step that runs.** Whichever of S1, S2,
S3 or S5 starts first replaces the two placeholder rows of the step table in
`thoughts/WB-38-frame-fingerprint/99-progress.md` with these nine, in this order, and sets
**Current step** to its own id; every later session updates its own row. With sessions in
parallel, one owner (the session that ran this first action) writes the file; the others
put their row, discoveries and verification results in their PR description, and the owner
copies them in (`05-implement.md`).

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

---

## Decisions this plan takes

The Structure left these to the Plan; each is used by the steps below.

| # | Decision | Why |
|---|---|---|
| P1 | The definition phrase S8 greps for is exactly `first-packet send to last-packet arrival`, on **one line** in `README.md`, in `internal/report/report.go` (the `OneWayDelayMethod` string) and in `internal/fingerprint/fingerprint.go` (the package comment) | the Structure's S8 Verify already uses it; one physical line so `grep -l` finds it |
| P2 | The `Method` line text is fixed in S5 (`OneWayDelayMethod`). It states the 1 s window, that a viewer's first frame and frames pending at stop are not counted, and that the CPU cost is not measured; it does **not** name NAL types, so H1's answer changes only `isVCL` | the Structure's S5 list; H1 stays one line |
| P3 | The per-viewer block type is `viewer.OneWayDelay`, in a **new file** `internal/viewer/onewaydelay.go`, plus one field in `viewer.Result`; S5 creates both. Everything that computes availability, reasons and pooling stays in `internal/report` | report imports viewer (`report.go:27`): a type the report package owned could not sit in `viewer.Result` without an import cycle. This mirrors packet transit (`viewer.go:54-77`). The Structure listed neither viewer file under S5; no parallel step touches package `viewer`, and S6 depends on S5 |
| P4 | A viewer's **first frame** — the one with the earliest RTP timestamp it ever saw — is never counted, complete or incomplete: its head cannot be proven (D4's seq first−1 rule) | otherwise every viewer reports `incompleteFrames ≥ 1` on a lossless path, S4's "no frame incomplete" cannot hold, and S7's `incompleteFrames > 0` proves nothing |
| P5 | A late packet whose timestamp belongs to a frame already completed or expired is ignored (it still counts as "seen" for its neighbours' head check) | otherwise an expired frame is re-opened by its late packet and counted incomplete twice |
| P6 | `invalid` counts both D3 aliasing and a match with no logged send of that clip index at or before t1 (the ring holds four loops); the package comment, the `Method` line and the README say both | both are "a match we will not sample", neither is "not in the clip" |
| P7 | Send log uses a `sync.RWMutex`, not atomics | ≈ 3,000 lookups/s at 100 viewers and 30 fps, one write per 33 ms; the Structure's fallback (atomics) stays in the risk table |
| P8 | The markerless relay test (S7) asserts per-viewer blocks only, never `Joined` or the aggregate: without markers no viewer joins (`rtpstats.go:212-252`), so the run has no verdict. S7 records this as a Discovery | changing the join rule is out of WB-38's scope |
| P9 | Splits: S3 → S3a (completion and t1) / S3b (expiry, markerless, rejection, bounds) at the test boundary given in S3; S6 → S6a (publisher and runner fill the log) / S6b (viewer, report wiring, end-to-end) at the boundary given in S6. Split only when the session passes 40% context | `03-structure.md`, Per-step risks |
| P10 | `CLAUDE.md` joins S8 (rule 5 says one-way delay is "not implemented yet", `CLAUDE.md:55-57`; the Layout lacks the two new packages, `:15-38`) | rule 1 of the repo: docs say what the code does |

---

## S1 · Fingerprint and the clip-side table

### Files touched

| Path | Action |
|---|---|
| `internal/fingerprint/fingerprint.go` | new — package comment (the definition), `Of`, `isVCL`, `Table` |
| `internal/fingerprint/fingerprint_test.go` | new |

### Changes

#### `internal/fingerprint/fingerprint.go`

**Package comment** (S1 owns it; S2's files carry only `package fingerprint`). It must say,
in this order: what one-way delay is — with the phrase `first-packet send to last-packet
arrival` whole on **one** comment line (P1); that the publisher logs t0 per absolute frame
index k before the frame's first packet is written; what the fingerprint is (below); that
a fingerprint shared by two clip frames is never sampled; that a match is the latest send
of the frame's clip index i = k mod N at or before t1, and a match is invalid when the RTP
timestamps prove it whole loops too new (D3) or when the log holds no send of that index at
or before t1 (P6). Imports: `crypto/sha256`, `encoding/binary`, `errors`,
`fmt`, `github.com/Allan-Nava/whipbench/internal/clip`.

**What to add:**

```go
// Of returns the fingerprint of one frame of the given codec (clip.VP8 or clip.H264):
// the first 64 bits, big-endian, of the SHA-256 of its hashed bytes. ok is false for an
// unknown codec and for an H.264 frame with no VCL NAL unit.
func Of(codec string, frame []byte) (fp uint64, ok bool)

// isVCL says which H.264 NAL unit types carry the coded picture (D1). It is the one
// line H1's check against H.264 Table 7-1 may change.
func isVCL(nalType byte) bool { return nalType >= 1 && nalType <= 5 }

// Status says what a fingerprint is in the clip.
type Status int

const (
	Unknown   Status = iota // in no clip frame: the frame counts as unmatched
	Unique                  // in exactly one clip frame: matched against the send log
	Duplicate               // in two or more clip frames: sent, never sampled (Q7)
)

// Table maps every clip frame's fingerprint to its index i in the loop. It is built
// once per run and only read afterwards, so every viewer shares one without a lock.
type Table struct{ /* codec string; ticks uint32; frames int; index map[uint64]int; dups map[uint64]struct{}; dupFrames int */ }

func NewTable(c *clip.Clip) (*Table, error)
func (t *Table) Lookup(fp uint64) (i int, st Status)
func (t *Table) Frames() int          // N = len(c.Frames)
func (t *Table) Ticks() uint32        // c.Ticks
func (t *Table) Codec() string        // c.Codec
func (t *Table) DuplicateFrames() int // clip frames whose fingerprint is not unique
```

**Expected behaviour:**
1. `Of(clip.VP8, b)`: SHA-256 over all of `b`; `ok` is true even for an empty `b`.
2. `Of(clip.H264, b)`: `clip.SplitAnnexB(b)`; skip empty units; keep each unit whose
   `unit[0] & 0x1F` satisfies `isVCL`, header byte included, in order; feed each kept
   unit to one `sha256` hash with no separator and no start code. No kept unit → `0,
   false`.
3. Any other codec → `0, false`.
4. The value is `binary.BigEndian.Uint64(sum[:8])`.
5. `NewTable(nil)` or a clip with no frames → error `fingerprint: the clip has no
   frames`. A frame `Of` refuses → error `fingerprint: clip frame <i> has nothing to
   hash`. Otherwise: fingerprints seen once go to `index` (fp → i); fingerprints seen
   twice or more go to `dups` and leave `index`; `dupFrames` counts every frame whose
   fingerprint is in `dups`.
6. `Lookup`: in `index` → `(i, Unique)`; in `dups` → `(-1, Duplicate)`; else `(-1,
   Unknown)`.

**Do NOT touch:** `internal/clip/clip.go` — `SplitAnnexB` is reused as it is.

### Tests (`internal/fingerprint/fingerprint_test.go`, package `fingerprint`)

| Case | Input | Expected |
|---|---|---|
| `TestFingerprintVP8WholeFrame` | `Of("vp8", []byte("abc"))` | `0xba7816bf8f01cfea, true` (the SHA-256 test vector: proves SHA-256, first 64 bits, big-endian) |
| `TestFingerprintVP8WholeFrame` | `Of("vp8", []byte("abd"))` | a different value, `true` |
| `TestFingerprintH264VCLOnly` | A = `00 00 00 01 67 AA` `00 00 00 01 68 BB` `00 00 01 06 CC` `00 00 00 01 65 DD EE` `00 00 01 41 FF`; B = `00 00 01 65 DD EE 00 00 00 01 41 FF` | `Of("h264", A) == Of("h264", B)`, both `ok`; equal to the SHA-256 prefix of bytes `65 DD EE 41 FF` |
| `TestFingerprintH264VCLOnly` (header byte kept) | B with `65` replaced by `45` | differs from `Of("h264", B)` |
| `TestFingerprintH264VCLOnly` (no VCL) | `00 00 00 01 67 AA 00 00 00 01 68 BB` | `0, false` |
| `TestFingerprintH264VCLOnly` (clip frames) | every frame of `clip.Load("h264", whipbench.ClipH264)`, rebuilt as `00 00 01` + each VCL unit only | same value as `Of("h264", frame.Data)` for all 120 |
| `TestFingerprintStableAcrossProcesses` | frame 0 and 1 of both embedded clips | VP8 `0xcc7dc033284a903a`, `0x624f3ffa2b4897e8`; H.264 `0x5000846a47ceb02b`, `0x25780bb4ae13506e` (golden values: a per-process seed, as `hash/maphash` has, would break them; a regenerated clip changes them — say so in the test comment) |
| `TestFingerprintUnknownCodec` | `Of("opus", []byte{1})` | `0, false` |
| `TestFingerprintIsVCL` | types 1, 2, 3, 4, 5 / 0, 6, 7, 8, 9, 24, 28 | `true` / `false` |
| `TestTableEmbeddedClips` | `NewTable` of each embedded clip | no error; `Frames() == 120`, `Ticks() == 3000`, `Codec()` is the codec, `DuplicateFrames() == 0`; for every i, `Lookup(Of(codec, Frames[i].Data)) == (i, Unique)` |
| `TestTableDuplicates` | a VP8 `clip.Clip{Codec: "vp8", Ticks: 3000, Frames: …}` of five frames with `Data` `"f0"`, `"f1"`, `"f2"`, `"f1"`, `"f4"` | `DuplicateFrames() == 2`; `Lookup(Of("vp8", []byte("f1"))) == (-1, Duplicate)`; `Lookup` of `"f4"` == `(4, Unique)`; `Lookup(42) == (-1, Unknown)` |
| `TestTableErrors` | `NewTable(nil)`; a clip with no frames; an H.264 clip whose frame is `00 00 01 67 AA` | each returns a non-nil error, the last naming frame 0 |

The test file imports `github.com/Allan-Nava/whipbench` for the embedded clips (checked: these expectations pass against a prototype on `74a7ad2`, 2026-10-03); an
in-package test importing the root package is fine (no cycle: the root imports nothing).

### Verify

```sh
go test -race -count=1 ./internal/fingerprint/ -run 'TestFingerprint|TestTable' -v; echo "exit $?"
grep -c 'first-packet send to last-packet arrival' internal/fingerprint/fingerprint.go   # prints 1
grep -c 'return nalType >= 1 && nalType <= 5' internal/fingerprint/fingerprint.go       # prints 1
```

### Acceptance criteria

- [ ] Every test case above passes, and the shared gate exits 0
- [ ] `go list -deps ./internal/fingerprint/` names no package outside the standard library and `internal/clip`
- [ ] Nothing imports `internal/fingerprint` yet

Test notation: `Lookup(x) == (i, Unique)` means both return values compared; every
`clip.Load`/`NewTable` error is checked with `t.Fatal`. `TestTableEmbeddedClips` loads each
embedded clip with `clip.Load(codec, bytes)` first.

---

## S2 · Send log and match with loop aliasing

### Files touched

| Path | Action |
|---|---|
| `internal/fingerprint/sendlog.go` | new — `SendLog` |
| `internal/fingerprint/sendlog_test.go` | new |
| `internal/fingerprint/match.go` | new — `Matcher`, `Verdict` |
| `internal/fingerprint/match_test.go` | new |

No package comment in these files (S1 owns it). S2 does not use any identifier from S1:
it takes the clip index i, not a fingerprint.

### Changes

#### `internal/fingerprint/sendlog.go`

```go
// SendLog is the publisher's send time t0 for each absolute frame index k, over the last
// four loops (D2). One goroutine records; any number look up.
type SendLog struct{ /* mu sync.RWMutex; frames uint64; k []uint64 (k+1, 0 = empty); t0 []time.Time; next uint64; loopMin time.Duration; haveLoop bool */ }

func NewSendLog(frames int) *SendLog
func (l *SendLog) Record(k uint64, t0 time.Time)
func (l *SendLog) Latest(i int, t1 time.Time) (k uint64, t0 time.Time, ok bool)
func (l *SendLog) Frames() int
func (l *SendLog) Recorded() uint64
func (l *SendLog) LoopMin() (time.Duration, bool)
```

**Expected behaviour:**
1. `NewSendLog(n)`: n ≤ 0 panics with `fingerprint: a send log needs at least one frame
   per loop` (a programming error: a clip always has frames). The ring has `4·n` slots;
   slot of k is `k % (4n)`.
2. `Record` on a **nil** `*SendLog` is a no-op — that is how `publish`, which keeps no
   log, stays as it is. Otherwise, under the write lock: store k and t0 in k's slot; if
   k ≥ n and slot `(k−n) % 4n` holds k−n, `d = t0.Sub(t0[k−n])` and `loopMin = min`;
   `next = max(next, k+1)`. t0 must keep its monotonic reading: store the `time.Time` as
   given, never `t0.Round(0)` or a Unix value.
3. `Latest(i, t1)`, under the read lock: `ok` false when `i < 0`, `i ≥ n` or nothing was
   recorded. Otherwise h = next−1; if h < i → not found; the first candidate is
   `k = h − (h − i) % n`, then `k − n`, `k − 2n`, … while k ≥ i and k > h − 4n (older k
   have been overwritten). Compute these bounds in `int64`, or test `h >= 4n` before
   subtracting: in `uint64`, `h − 4n` and `k − n` wrap for a young log, and
   `TestSendLogLatest` (h = 8, 4n = 12) fails. Return the first candidate whose slot still holds that k and
   whose t0 is **not after** t1 (`!t0.After(t1)`); none → `0, time.Time{}, false`.
4. `Frames()` = n; `Recorded()` = next (one past the highest k, i.e. frames logged when
   the publisher logs every k from 0); `LoopMin()` = `loopMin, haveLoop`, under the read
   lock.

#### `internal/fingerprint/match.go`

```go
// Verdict is what a viewer does with one matched frame.
type Verdict int

const (
	Sampled   Verdict = iota + 1 // t1 − t0 is a sample
	Aliased                      // invalid: the RTP timestamps prove the match whole loops too new (D3)
	NotLogged                    // invalid: no send of that clip index at or before t1 in the log
)

// Matcher is one viewer's matching state: the log it reads and the anchor (kA, tsA) of
// its last sampled match. Not safe for concurrent use; one per viewer.
type Matcher struct{ /* log *SendLog; ticks int64; frames int64; have bool; kA uint64; tsA uint32 */ }

func NewMatcher(log *SendLog, ticks uint32) *Matcher
func (m *Matcher) Match(i int, ts uint32, t1 time.Time) (Verdict, time.Duration)
```

**Expected behaviour of `Match`:**
1. `kL, t0, ok := m.log.Latest(i, t1)`; not ok → `NotLogged, 0`, anchor unchanged.
2. No anchor yet → `Sampled, t1.Sub(t0)`; anchor = (kL, ts).
3. With an anchor: `dts := int64(int32(ts − m.tsA))` (uint32 subtraction, then the int32
   cast: RTP wrap handled). If `dts % ticks != 0` → no evidence: `Sampled`, anchor =
   (kL, ts). Else `Δ = dts / ticks`, `diff = int64(kL) − (int64(m.kA) + Δ)`. If `diff ≥
   frames` and `diff % frames == 0` → `Aliased, 0`, anchor **unchanged**. Otherwise
   (diff 0, negative, or not a whole number of loops) → `Sampled, t1.Sub(t0)`, anchor =
   (kL, ts).
4. The sample is `t1.Sub(t0)`: monotonic whenever both carry a monotonic reading, which
   `time.Now()` values do.

### Tests

`sendlog_test.go` (package `fingerprint`); a helper `at(ms int) time.Time` returns
`base.Add(time.Duration(ms) * time.Millisecond)` with `base := time.Now()`.

| Case | Input | Expected |
|---|---|---|
| `TestSendLogLatest` | n = 3; Record k = 0..8 at 33·k ms | `Latest(1, at(140))` → k 4; `Latest(1, at(300))` → k 7; `Latest(2, at(70))` → k 2; `Latest(0, at(0))` → k 0 (equal t0 counts) |
| `TestSendLogNeverReturnsAnUnloggedSend` | same log | `Latest(2, at(65))` → `ok` false (k 2 is sent at 66 ms, nothing earlier for i 2) |
| `TestSendLogRingOverwrites` | n = 3 (12 slots); Record k = 0..14 at 33·k ms | `Latest(0, at(10))` → `ok` false (k 0 overwritten by k 12, never returned); `Latest(0, at(100))` → k 3; `Latest(0, at(1000))` → k 12 |
| `TestSendLogBadIndex` | n = 3, one record | `Latest(-1, …)`, `Latest(3, …)` → `ok` false; empty log → `ok` false |
| `TestSendLogNilRecordIsANoOp` | `var l *SendLog; l.Record(0, time.Now())` | no panic |
| `TestSendLogNewPanicsOnZero` | `NewSendLog(0)` in a `defer recover()` | it panics |
| `TestSendLogLoopMin` | n = 4; Record k = 0..3 at 10·k ms | `LoopMin()` → `0, false`; then k = 4 at 50 ms → `50ms, true`; k = 5 at 55 ms → `45ms, true` |
| `TestSendLogSlip` | n = 4; k = 0..11 at 10·k ms, every k ≥ 6 shifted by +600 ms (a slip) | `Recorded() == 12`; `LoopMin() == 40ms` (a loop that does not span the slip; those that do take 640 ms); `Latest(2, at(700))` → k 10 (the index follows k, not time) |
| `TestSendLogConcurrent` | n = 120; one goroutine Records k = 0..2399 with `time.Now()`; eight goroutines call `Latest(k%120, time.Now())` 5,000 times each | runs clean under `-race`; every `ok` result has `k % 120 == i` and `!t0.After(t1)` |

`match_test.go` (package `fingerprint`, so it may read `m.have`, `m.kA`, `m.tsA`): every
case builds its **own** log and matcher; n = 10, ticks = 3000, k = 0..39 recorded at 33·k
ms (and any "Record k = a..b" below also at 33·k ms), and `T := at(100000)` (after every
send, so kL is simply the highest recorded k ≡ i). "Anchor (35, 1000)" means a first
`Match(5, 1000, T)` on a fresh matcher.

| Case | Input | Expected |
|---|---|---|
| `TestMatchFirstIsKept` | fresh matcher, `Match(5, 1000, T)` | `Sampled`, d = `T.Sub(t0[35])` = 98,845 ms; anchor (35, 1000) |
| `TestMatchAliased` | anchor (35, 1000); Record k = 40..49; `Match(9, 1000+4·3000, T)` | kL 49 = 35 + 4 + 10 → `Aliased, 0`; anchor still (35, 1000); the same call again is `Aliased` again |
| `TestMatchTwoLoops` | anchor (35, 1000); Record k = 40..59; `Match(7, 1000+2·3000, T)` | kL 57 = 35 + 2 + 20 → `Aliased` (m = 2) |
| `TestMatchNoEvidence` | anchor (35, 1000); `Match(6, 1000+4500, T)` | 4500 is not a whole number of ticks → `Sampled`; anchor (36, 5500) |
| `TestMatchNotAWholeLoop` | anchor (35, 1000); `Match(9, 1000+3·3000, T)` | kL 39, expected 38, diff 1 → `Sampled`; anchor (39, 10000) |
| `TestMatchTimestampWrap` | `w := uint32(0xFFFFF000)` (a variable: the constant expression `0xFFFFF000+6000` does not compile as a uint32); fresh matcher, `Match(5, w, T)`; then `Match(7, w+6000, T)` (wraps to `0x770`) | Δ = 2, kL 37, diff 0 → `Sampled`. Second matcher: same first call, Record k = 40..47, same second call → kL 47, diff 10 → `Aliased` |
| `TestMatchNotLogged` | fresh matcher, `Match(3, 0, at(50))` | k ≡ 3 first sent at 99 ms → `NotLogged, 0`; `m.have` still false |
| `TestMatchSampleIsMonotonic` | n = 10, only k 0 recorded at `t0 := time.Now()`; `Match(0, 0, t0.Add(5*time.Millisecond))` | `Sampled`, exactly `5ms` |

Each case states the k its lookup must return, so a wrong `Latest` fails the match tests
too. Every number in both tables was checked against a prototype on `74a7ad2` (2026-10-03).

### Verify

```sh
go test -race -count=1 ./internal/fingerprint/ -run 'TestSendLog|TestMatch' -v; echo "exit $?"
```

### Acceptance criteria

- [ ] Every test case above passes under `-race`, and the shared gate exits 0
- [ ] `sendlog.go` and `match.go` declare no package comment and use nothing from `fingerprint.go`
- [ ] Still imported by nothing

---

## S3 · The reassembler

### Files touched

| Path | Action |
|---|---|
| `internal/reassembler/reassembler.go` | new |
| `internal/reassembler/reassembler_test.go` | new |

### Changes

#### `internal/reassembler/reassembler.go`

**Package comment:** packets plus their arrival in, complete frames out, one reassembler
per viewer; why not pion's samplebuilder (D4: it pops a frame only when the next arrives,
records no arrival, gives up by sequence count); the completeness rule, the 1 s window, the
frame-end rule (D5), and that completion instants are kept for WB-41 (D6). Imports: `fmt`,
`sort`, `time`, `internal/clip`, `github.com/pion/rtp`, `github.com/pion/rtp/codecs`.

```go
// Window is how long a frame may stay incomplete after its first packet arrived. The
// one-way delay Method line in internal/report states it; S7 observes whether it
// outlasts pion's NACK retries.
const Window = time.Second

// Frame-end rules, as the report's frameEnd spells them (D5).
const (
	EndMarker    = "marker"
	EndTimestamp = "timestamp"
)

// Frame is one complete frame.
type Frame struct {
	Timestamp uint32
	Payload   []byte    // depacketised: the VP8 frame, or H.264 Annex-B (4-byte start codes); nil when Rejected
	Rejected  bool      // the depacketiser refused it (STAP-B, MTAP, FU-B…): a server re-packetised it
	Arrival   time.Time // t1: the arrival of the frame's last packet, its own arrival (D5)
	Completed time.Time // when the reassembler found it complete; unused until WB-41 (D6)
}

// Reassembler is one viewer's frame reassembly. Not safe for concurrent use.
type Reassembler struct{ /* see "State" below */ }

func New(codec string) (*Reassembler, error)
func (r *Reassembler) Push(p *rtp.Packet, arrival time.Time) []Frame
func (r *Reassembler) Incomplete() uint64
func (r *Reassembler) FrameEnd() string
```

**State** (unexported; the bounded-state test reads the three maps by these names):
`newDep func() rtp.Depacketizer`; extended sequence numbers `haveSeq bool, maxExt
uint64`; `seen map[uint64]seen` (ext seq → `{ts uint32; at time.Time}`, every packet
recently received, including those of closed frames); `pending map[uint32]*pending` (by
RTP timestamp: `pkts map[uint64]packet` with a **copy** of the payload, the marker and the
arrival; `lo, hi uint64`; `first time.Time`, the arrival of its first pushed packet);
`closed map[uint32]time.Time` (timestamps completed or expired, with when); `earliest
uint32, haveEarliest bool` (the earliest RTP timestamp ever seen, by `int32` difference);
`end string` (`""` until the first frame completes); `incomplete uint64`; `lastPrune
time.Time`.

**Expected behaviour:**
1. `New`: `clip.VP8` → `&codecs.VP8Packet{}`, `clip.H264` → `&codecs.H264Packet{}` (a
   **fresh** one per frame through `newDep`); any other codec → error `reassembler: no
   depacketiser for codec %q`.
2. `Push(p, arrival)`, in this order; `arrival` is the only clock (injected in tests):
   1. **Expire**: every pending frame with `arrival.Sub(first) >= Window` is deleted and
      its timestamp put in `closed`; it counts in `incomplete` **unless** its timestamp
      equals `earliest` — a viewer's first frame, whose head it cannot prove (P4).
   2. **Extend** the sequence number: the first packet gets `uint64(seq) + 1<<32`; later
      ones `uint64(int64(maxExt) + int64(int16(seq − uint16(maxExt))))`, and `maxExt`
      rises to it.
   3. **Duplicate**: an ext seq already in `seen` → return nil (the first copy wins).
   4. Record `seen[ext] = {ts, arrival}`; move `earliest` back when `int32(ts − earliest)
      < 0`.
   5. A timestamp in `closed` → the packet joins no frame (P5). Otherwise add it to
      `pending[ts]` (created with `first = arrival`), copying the payload; widen `lo`/`hi`.
   6. **Complete**: walk every pending frame in ascending `lo`; a frame is complete when
      (a) it holds `hi − lo + 1` packets; (b) `seen[lo−1]` exists with `int32(ts(lo−1) −
      ts) < 0` — the head is proven; (c) its end is known: while `end != EndTimestamp`,
      the packet at `hi` carries the marker (set `end = EndMarker`), **or** `seen[hi+1]`
      exists with a later timestamp and no packet of the frame carries a marker (set
      `end = EndTimestamp` — the switch, never undone); once `end == EndTimestamp`, only
      the second condition counts and a marker is ignored. A complete frame leaves
      `pending`, enters `closed`, and is depacketised: one fresh depacketiser,
      `Unmarshal` of each payload from `lo` to `hi`, outputs appended; any error →
      `Rejected: true, Payload: nil`. `Arrival` = the arrival of the packet at `hi`;
      `Completed` = this push's `arrival`.
   7. **Prune** when `arrival.Sub(lastPrune) >= Window`: drop `seen` and `closed` entries
      older than `2·Window`; set `lastPrune = arrival`.
   8. Return the completed frames in ascending `lo` order, nil when none.
3. There is no `Flush`/`Close`: frames pending when the viewer stops are counted nowhere.
4. `Incomplete()` returns the counter; `FrameEnd()` returns `end`.

### Tests (`internal/reassembler/reassembler_test.go`, package `reassembler`)

Helpers: `base := time.Unix(1000, 0)`; `at(ms float64) time.Time`; `pk(seq uint16, ts
uint32, marker bool, payload ...byte) *rtp.Packet` (Version 2); VP8 payloads `s(b) =
{0x10, b}` (S bit, first packet) and `c(b) = {0x00, b}` (continuation) — `VP8Packet`
returns the byte after the one-byte descriptor. "Prime" = `Push(pk(9, 0, true, s('p')...),
at(0))`, a first frame that never completes and is never counted, whose seq 9 proves the
next frame's head.

| Case | Input (after the prime unless stated) | Expected |
|---|---|---|
| `TestCompleteOnMarker` | 10 `s('a')` @10, 11 `c('b')` @11, 12 M `c('c')` @12, all ts 3000 | first two pushes return nil; the third returns one `Frame{Timestamp: 3000, Payload: "abc", Arrival: at(12), Completed: at(12)}`; `FrameEnd() == "marker"` |
| `TestFirstCopyWins` | 10 `s('a')` @10, 12 M `c('c')` @12, 12 M `c('X')` @13, 11 `c('b')` @14, 12 M `c('Y')` @15 | the duplicate pushes return nil; the push of 11 returns `"abc"` with `Arrival` at(12) (first copy) and `Completed` at(14) — reorder and the completion instant in one case |
| `TestSequenceWrap` | no prime: 65534 M `s('p')` ts 0 @0; 65535 `s('a')` @10, 0 `c('b')` @11, 1 M `c('c')` @12, ts 3000 | one frame `"abc"` |
| `TestLostHeadNeverCompletes` | A (ts 3000): 11 `c('b')` @10, 12 M `c('c')` @11 — 10 lost; B (6000): 13 `s('d')` @20, 14 M `c('e')` @21; C (9000): 15 M `s('f')` @1010 | A never returned; B returned `"de"`; `Incomplete() == 0` before the push at 1010, `1` after it (A); C returned |
| `TestHeadWithLaterTimestamp` | no prime: 9 M `s('p')` ts **6000** @0; then 10 `s('a')` @1, 11 `c('b')` @2, 12 M `c('c')` @3 at ts 3000 | nothing returned: seq 9 is not an earlier timestamp |
| `TestIncompleteAfterWindow` | A (3000): 10 `s('a')` @5, 12 M `c('c')` @7 (11 lost); 13 M `s('d')` ts 6000 @1004; 14 M `s('e')` ts 9000 @1005; late 11 `c('b')` ts 3000 @1006; 15 M `s('f')` ts 12000 @2100 | after @1004 (999 ms): `Incomplete() == 0`; after @1005 (1000 ms): `1`; the late 11 returns nil and `Incomplete()` stays 1, also after @2100; 13, 14 and 15 each come out complete |
| `TestStaleFUBytesNeverHashed` | codec h264; prime `pk(9, 0, true, 0x41, 0x01)`; A (3000): 10 `7C 85 'x'` @1 (FU-A start), 11 `7C 05 'y'` lost, 12 M `7C 45 'z'` @3 (FU-A end); B (6000): 13 M `41 'b'` @4 | only B comes out, `Payload == 00 00 00 01 41 'b'` exactly |
| `TestMarkerlessSwitch` | A (3000): 10 `s('a')` @1, 11 M `c('b')` @2; B (6000): 12 `s('c')` @20, 13 `c('d')` @21 (no marker); C (9000): 14 `s('e')` @30, 15 M `c('f')` @31; D (12000): 16 `s('g')` @40 | A out on 11, `FrameEnd() "marker"`; B out on the push of 14 with `Arrival` at(21), `FrameEnd() "timestamp"`; the push of 15 returns nil (marker ignored after the switch); C out on the push of 16, `"ef"`, `Arrival` at(31) |
| `TestLostMarkerDoesNotSwitch` | A (3000): 10 `s('a')` @1, 11 M `c('b')` @2; B (6000): 12 `s('c')` @3, 13 M lost; C (9000): 14 `s('e')` @5, 15 M `c('f')` @6; D (12000): 16 `s('g')` @8, 17 M `c('h')` @9; E (15000): 18 M `s('i')` @1010 | D out `"gh"` (its head, 15, arrived); C never (its head, 13, is lost); after @1010 `Incomplete() == 2` (B, C), E out, `FrameEnd()` still `"marker"` |
| `TestRejected` | codec h264; prime `pk(9, 0, true, 0x41, 0x01)`; single-packet M frames 10 ts 3000 `19 00 01` (STAP-B), 11 ts 6000 `1A 00 01` (MTAP16), 12 ts 9000 `1D 00 01` (FU-B) | each push returns one frame with `Rejected` true and `Payload` nil |
| `TestFirstFrameNotCounted` | no prime: 10 `s('a')` @0, 11 M `c('b')` @1 (ts 3000); 12 M `s('c')` ts 6000 @1500 | the first frame never comes out; the push at 1500 returns `"c"`; `Incomplete() == 0` |
| `TestPendingAtStopNotCounted` | 10 `s('a')` ts 3000 @1, then nothing | `Incomplete() == 0` |
| `TestStateIsBounded` | no prime; 1,800 frames (60 s at 30 fps), ts 3000·k, three packets each (`s`, `c`, `c` with M on the third), arrivals k·1000/30 ms + j·0.1 ms; the middle packet of every frame with k % 10 == 5 is lost (its seq still consumed) | 1,619 frames out (1,800 − 180 lossy − the first); `Incomplete() == 177` (the three last lossy frames are still pending); during the run `len(r.seen) < 400`, `len(r.closed) < 200`, `len(r.pending) < 40` (measured maxima 261, 91, 5) |
| `TestNewCodec` | `New("opus")`; `New("vp8")`; `New("h264")` | error; no error; no error |

Every row was checked against a prototype on `74a7ad2` (2026-10-03). A row's `@n` is the
arrival `at(n)`; a packet without a payload named in its row does not occur.

**Split if the session passes 40% (P9)** — branches `wb-38/s3a`, `wb-38/s3b`: S3a = the file plus `TestCompleteOnMarker`,
`TestFirstCopyWins`, `TestSequenceWrap`, `TestLostHeadNeverCompletes`,
`TestHeadWithLaterTimestamp`, `TestNewCodec` — verify with `-run
'TestCompleteOnMarker|TestFirstCopyWins|TestSequenceWrap|TestLostHead|TestHeadWith|TestNewCodec'`;
S3b = the remaining eight tests and any behaviour they need; verify with the full command
below.

### Verify

```sh
go test -race -count=1 ./internal/reassembler/ -v; echo "exit $?"
grep -c 'const Window = time.Second' internal/reassembler/reassembler.go   # prints 1
```

### Acceptance criteria

- [ ] Every test case above passes, and the shared gate exits 0
- [ ] `Push` copies payload bytes (no slice of `p.Payload` survives the call)
- [ ] Imported by nothing yet

---

## S4 · The clip round-trip test the Design keeps

### Files touched

| Path | Action |
|---|---|
| `internal/reassembler/roundtrip_test.go` | new, package `reassembler_test` |

### Changes

One test, `TestRoundTrip`, with two subtests named exactly `H264` and `VP8` (`t.Run`), each:

1. `c, err := clip.Load(codec, data)` with `data` = `whipbench.ClipH264` or
   `whipbench.ClipVP8`; `table, err := fingerprint.NewTable(c)`; `r, err :=
   reassembler.New(codec)` — each error `t.Fatal`.
2. One payloader for the whole subtest, built as the publisher builds it
   (`internal/publisher/publisher.go:204-210`): `&codecs.H264Payloader{}` or
   `&codecs.VP8Payloader{EnablePictureID: true}`; payload size `publisher.MTU`.
3. `seq := uint16(65000)`, `base := uint32(0xFFFFF000)` (both wrap during the test),
   `start := time.Unix(1000, 0)`. For k = 0 … 2N **inclusive** (2N + 1 frames; frame 0 is
   the viewer's first frame and never comes out, P4): `ts := c.Timestamp(base, k)`;
   payloads `pay.Payload(publisher.MTU, c.Frame(uint64(k)).Data)` (`MTU` is an untyped constant); packet j gets `SequenceNumber
   seq` (then `seq++`), `Timestamp ts`, `Marker j == last`, arrival `start.Add(time.Duration(k)*c.FrameDuration() +
   time.Duration(j)*time.Microsecond)` (`clip.go:43`, 33.3 ms); collect every frame `Push` returns. For H.264 count packets whose
   `payload[0]&0x1F == 24` (STAP-A).
4. Assert: exactly 2N frames came out; none `Rejected`; for each, `fingerprint.Of(codec,
   f.Payload)` is ok and `table.Lookup` returns `(int((f.Timestamp−base)/c.Ticks) % N,
   Unique)` — the uint32 subtraction handles the wrap; `r.Incomplete() == 0`; H.264 STAP-A
   count == 9 (one per keyframe, k = 0, 30, …, 240). Log `"%s: %d of %d frames matched;
   %d STAP-A"` with `t.Logf`.

**Do NOT touch:** `internal/publisher/publisher.go` (S6 does, in parallel); the test only
reads `publisher.MTU`.

### Tests

| Case | Input | Expected |
|---|---|---|
| `TestRoundTrip/H264` | embedded H.264 clip, k = 0..240 | 240/240 frames matched, 9 STAP-A packets, 0 incomplete, 0 rejected |
| `TestRoundTrip/VP8` | embedded VP8 clip, k = 0..240 | 240/240 frames matched, 0 incomplete, 0 rejected |

Checked against the S1 and S3 prototypes on `74a7ad2` (2026-10-03): both 240/240, 9 STAP-A.

### Verify

```sh
go test -race -count=1 ./internal/reassembler/ -run TestRoundTrip -v; echo "exit $?"
go test -race -count=1 ./internal/reassembler/ -run TestRoundTrip -v | grep -cE -- '--- PASS: TestRoundTrip.*(H264|VP8)'   # prints 2
```

### Acceptance criteria

- [ ] Both subtests pass; the grep prints 2; the shared gate exits 0

---

## S5 · Report blocks, Markdown, stdout and the two name guards

### Files touched

| Path | Action |
|---|---|
| `internal/viewer/onewaydelay.go` | new — the per-viewer block type (P3) |
| `internal/viewer/viewer.go` | modify — **one field** in `Result`, nothing else (P3) |
| `internal/report/report.go` | modify |
| `internal/report/markdown.go` | modify |
| `internal/report/report_test.go` | modify |
| `internal/metrics/metrics_test.go` | modify — the message only |
| `cmd/whipbench/main.go` | modify — the stdout headline |
| `cmd/whipbench/main_test.go` | modify |

### Changes

#### `internal/viewer/onewaydelay.go` (new, package `viewer`)

```go
// SourceFingerprint is the source of WB-38's send instant: the publisher's log, found by
// the frame's fingerprint. WB-39 adds "stamp" beside it under the same key (D7).
const SourceFingerprint = "fingerprint"

// OneWayDelay is one viewer's one-way delay from one source (D7). The viewer fills the
// counts, FrameEnd and Hist, and a Reason only when it could not measure at all (S6); the
// report decides Available, Samples, Ms and every other Reason, so a block built anywhere
// is finished one way.
type OneWayDelay struct {
	Source           string           `json:"source"`
	Available        bool             `json:"available"`
	Reason           string           `json:"reason,omitempty"`
	FrameEnd         string           `json:"frameEnd,omitempty"` // "marker" or "timestamp" (D5)
	CompleteFrames   uint64           `json:"completeFrames"`
	IncompleteFrames uint64           `json:"incompleteFrames"`
	Samples          uint64           `json:"samples"`
	Invalid          uint64           `json:"invalid"`
	UnmatchedFrames  uint64           `json:"unmatchedFrames"`
	Ms               *stats.Summary   `json:"ms,omitempty"`
	Hist             *stats.Histogram `json:"-"` // the samples in ms; exported so the report and its tests can pool it
}
```

Accounting the doc comment states: `completeFrames = samples + invalid + unmatchedFrames +`
frames whose fingerprint is a clip duplicate.

#### `internal/viewer/viewer.go`

**Where:** `Result`, between `RTP` (`:96`) and `PacketTransit` (`:97`):
`OneWayDelay []OneWayDelay `json:"oneWayDelay"``. Nothing else in this file changes in S5.

#### `internal/report/report.go`

1. **`Aggregate`** (`:80-106`): add `OneWayDelay []OneWayDelay `json:"oneWayDelay"`` just
   before `PacketTransit` (`:105`) — JSON order is field order, and the headline goes first.
2. **New types and constants**, after `Aggregate`:

```go
// OneWayDelay is the pooled one-way delay of one source over the viewers that joined.
type OneWayDelay struct {
	Source            string         `json:"source"`
	Available         bool           `json:"available"`
	Reason            string         `json:"reason,omitempty"`
	Viewers           int            `json:"viewers"`
	ViewersByFrameEnd map[string]int `json:"viewersByFrameEnd"` // never nil
	CompleteFrames    uint64         `json:"completeFrames"`
	IncompleteFrames  uint64         `json:"incompleteFrames"`
	Samples           uint64         `json:"samples"`
	Invalid           uint64         `json:"invalid"`
	UnmatchedFrames   uint64         `json:"unmatchedFrames"`
	DuplicateFrames   int            `json:"duplicateFrames"`
	LoopFrames        int            `json:"loopFrames"`
	LoopMinMs         *float64       `json:"loopMinMs,omitempty"`
	Ms                *stats.Summary `json:"ms,omitempty"`
}

// Fingerprint is what run's clip table and send log add to the pooled block; nil when this
// process published nothing (view, or a run without a WHIP endpoint).
type Fingerprint struct {
	DuplicateFrames int
	LoopFrames      int
	LoopMin         time.Duration // 0 until the log has seen a whole loop
}

// NoSendLogReason: without a send log in this process there is no t0 to subtract (Q2).
const NoSendLogReason = "no send log: the send log lives in the `run` process that publishes the clip, and this process published nothing"

// NoTrackReason: the viewer never received a video track, so it reassembled nothing.
const NoTrackReason = "no video track reached this viewer"

// FingerprintDelay returns the pooled fingerprint block; Build always produces one. An
// Aggregate built by hand without one gets {Source: "fingerprint", Reason: NoSendLogReason}.
func (a Aggregate) FingerprintDelay() OneWayDelay

// OneWayDelayMethod is the definition every report carries (P1, P2).
const OneWayDelayMethod = "…" // text below
```

   No reason or constant except `OneWayDelayMethod` may contain "one-way delay" (D8).
3. **`Input`** (`:117-129`): add `Fingerprint *Fingerprint` after `Interrupted`, with a
   comment that `runner` sets it when it publishes (S6).
4. **`Build`** (`:132-171`): right after the sort at `:159`, normalise every viewer —
   `r.Viewers[i].OneWayDelay = normaliseOneWayDelay(r.Viewers[i].OneWayDelay,
   in.Fingerprint)` — then pass `in.Fingerprint` to `aggregate` (`:165`).

```go
// normaliseOneWayDelay gives every viewer exactly its blocks, each finished; a viewer with
// none gets one unavailable fingerprint block saying why.
func normaliseOneWayDelay(bs []viewer.OneWayDelay, fp *Fingerprint) []viewer.OneWayDelay

// finishOneWayDelay decides Available, Reason, Samples and Ms from the counts; idempotent.
func finishOneWayDelay(o viewer.OneWayDelay) viewer.OneWayDelay
```

   `normaliseOneWayDelay`: work on `slices.Clone(bs)`; when empty, one block `{Source:
   viewer.SourceFingerprint, Reason: NoSendLogReason}` if `fp == nil`, else `Reason:
   NoTrackReason`; then `finishOneWayDelay` on each. `finishOneWayDelay`, in order: empty
   `Source` → `viewer.SourceFingerprint`; `Hist != nil` → `Samples = Hist.Count()`; a
   non-empty `Reason` → `Available false, Ms nil`, return; `Samples == 0` → `Available
   false, Ms nil, Reason = fmt.Sprintf("no valid sample: %d complete frames (%d unmatched,
   %d invalid), %d incomplete", CompleteFrames, UnmatchedFrames, Invalid,
   IncompleteFrames)`, return; else `Available true`, `sm := o.Hist.Summary(); o.Ms = &sm`
   (as at `:223-224`). A block with `Hist == nil` has `Samples` left as given and, when
   `Samples > 0`, is treated as zero samples — never dereference a nil `Hist`.
5. **`aggregate`** becomes `func aggregate(target int, vs []viewer.Result, pub
   *publisher.Result, fp *Fingerprint) Aggregate`. Beside the packet-transit pooling, for
   each **joined** viewer take its first block with `Source == viewer.SourceFingerprint`:
   add its four counts (`CompleteFrames`, `IncompleteFrames`, `Invalid`,
   `UnmatchedFrames`) to the pooled block; when `Available && Hist != nil`, `Merge` its
   `Hist` into one pooled histogram, `Viewers++`, `ViewersByFrameEnd[FrameEnd]++`; else
   collect its `Reason`. After the loop: `Samples = pooled.Count()`; when `fp != nil` copy
   `DuplicateFrames`, `LoopFrames`, and `LoopMinMs = LoopMin in ms` when `LoopMin > 0`.
   Reason, first match wins: `fp == nil` → `NoSendLogReason`; samples > 0 → `Available`,
   `Ms`, and when some joined viewers had none, `fmt.Sprintf("pooled over %d of %d joined
   viewers; the other %d had none (%s)", …, mostCommon(reasons))` — the packet-transit
   wording at `:226`; no viewer joined → `"no viewer joined"`; else `mostCommon(reasons)`.
   `a.OneWayDelay = []OneWayDelay{pooled}`.
6. **`Method`** (`:289-297`): insert `OneWayDelayMethod` as a new element **before** the
   packet transit line (`:294`). Leave `:294` and `:295` untouched. The text, one line:

> One-way delay (source fingerprint): per frame, first-packet send to last-packet arrival, on the monotonic clock of the one process that runs both ends. The publisher logs t0 just before it hands a frame's first packet to the stack, by absolute frame index. Each viewer reassembles frames by RTP timestamp; t1 is the arrival of the frame's last packet — the marker packet, or, on a stream without markers, the last before the next timestamp (frameEnd). A complete frame is hashed — the first 64 bits of SHA-256 over the whole VP8 frame, or over the H.264 VCL NAL units — and matched to the latest send of that clip frame at or before t1; the sample is t1 − t0. A frame still incomplete 1 s after its first packet counts in incompleteFrames and is never hashed; a viewer's first frame and the frames still pending when it stops are not counted. A frame the depacketiser rejects or that is not in the clip counts in unmatchedFrames; a frame whose bytes repeat in the clip (duplicateFrames) is never sampled; a match the RTP timestamps prove whole loops too new, or one with no logged send at or before t1, is invalid. loopMinMs is the shortest time the publisher took to send loopFrames frames: a delay longer than that is caught only by that RTP timestamp check, and a viewer's first match is taken as it is. Retransmitted packets count like any other. Network plus server forwarding plus both clients' stacks — not glass-to-glass; a `view` run has no send log and reports it unavailable. The CPU cost of reassembling and hashing every frame on every viewer is not measured.

#### `internal/report/markdown.go`

`b := a.FingerprintDelay()` once, inside `if a.Valid` (`:44`).
1. First row of the aggregate table, before `row("join: first keyframe", …)` (`:58`):
   when `b.Available && b.Ms != nil`, `row("one-way delay (fingerprint, per frame)",
   *b.Ms, " ms")`.
2. After the table's `w("\n")` (`:67`) and **before** the packet-transit lines (`:68-72`):
   `!b.Available` → `w("**One-way delay: unavailable** — %s.\n\n", esc(b.Reason))`;
   otherwise `w("One-way delay (fingerprint): %d samples from %d complete frames on %d
   viewers (frame end: %s); %d incomplete, %d unmatched, %d invalid; %d duplicate clip
   frames never sampled; loop %d frames%s.\n\n", …)` where frame end is the sorted
   `ViewersByFrameEnd` as `"marker 4, timestamp 1"` and the last `%s` is `", sent in %.0f
   ms at the fastest"` when `LoopMinMs != nil`, else `""`; then, when `b.Reason != ""`,
   `w("One-way delay %s.\n\n", esc(b.Reason))`.
3. Viewers table (`:105-106`): insert the columns `delay p50 | delay p99` before
   `transit p50`, alignment `---:|---:|`; in each row (`:112-122`) the viewer's first
   fingerprint block's `Ms.P50`/`Ms.P99` as `"%.1f ms"` when available, else `"n/a"`.

#### `cmd/whipbench/main.go`

Replace the body of `if a.Valid { … }` (`:305-310`) with `fmt.Fprint(stdout,
headline(a))` and add:

```go
// headline is the two lines a valid run prints after its verdict: one-way delay first
// (the figure servers are ranked by), then join, loss and packet transit.
func headline(a report.Aggregate) string
```

Line 1: `b := a.FingerprintDelay()`; available with `Ms` → `"one-way delay
(fingerprint) p50 %.1f ms, p99 %.1f ms\n"`; else `"one-way delay unavailable: %s\n"` with
`b.Reason`. Line 2: the existing text of `:305-310` unchanged — `"join (first keyframe)
p50 %.0f ms, p95 %.0f ms; loss %.3f%%; "` then the packet-transit clause and `"\n"`.

#### `internal/metrics/metrics_test.go`

`:41-43`: keep the assertion; the message becomes `"no one_way_delay series until it
carries a source label (fingerprint now, stamp with WB-39): an unlabelled series would
change meaning when the second source lands:\n%s"`.

**Do NOT touch:** `Schema` (`report.go:32`), `PacketTransit` on either side, the packet
transit `Method` line, `internal/metrics/metrics.go`.

### Tests

`internal/report/report_test.go` gains a helper `hist(vs ...float64) *stats.Histogram`
(`stats.NewHistogram` plus `Add` of each value) and these cases. Each builds with
`Build(Input{Command: "run", Scenario: sc(n), Viewers: vs, StartedAt: now, FinishedAt:
now, Fingerprint: …})`, `now := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)`; `sc` and
`viewers` are the existing helpers (`:18-41`); a viewer's block is set as
`vs[i].OneWayDelay = []viewer.OneWayDelay{{…}}`.

| Case | Input | Expected |
|---|---|---|
| `TestOneWayDelayBlocks` | `Build` with `Fingerprint{LoopFrames: 120, LoopMin: 3990*time.Millisecond}` and `viewers(3, 0)`, viewer 0 given `{Source: "fingerprint", FrameEnd: "marker", CompleteFrames: 2, Hist: hist(10, 20)}` | JSON has `"oneWayDelay": [` 4 times (3 viewers + aggregate) and each of `"source": "fingerprint"`, `"frameEnd": "marker"`, `"completeFrames"`, `"incompleteFrames"`, `"samples"`, `"invalid"`, `"unmatchedFrames"`, `"viewersByFrameEnd"`, `"duplicateFrames"`, `"loopFrames": 120`, `"loopMinMs": 3990`; `r.Schema == "whipbench.report/v0"`; `"packetTransit": {` still 4 times |
| `TestOneWayDelayPools` | as above plus viewer 1 `{FrameEnd: "timestamp", CompleteFrames: 1, Hist: hist(30)}` and viewer 2 `{CompleteFrames: 5, UnmatchedFrames: 5, Hist: hist()}` | pooled: `Available`, `Samples 3`, `Ms.N 3`, `Ms.Min 10`, `Ms.Max 30`, `Viewers 2`, `ViewersByFrameEnd {"marker": 1, "timestamp": 1}`, `CompleteFrames 8`, `UnmatchedFrames 5`, `Reason "pooled over 2 of 3 joined viewers; the other 1 had none (no valid sample: 5 complete frames (5 unmatched, 0 invalid), 0 incomplete)"` |
| `TestOneWayDelayZeroSamplesIsNeverANumber` | viewer 2 of the case above alone (`viewers(1, 0)`, `Fingerprint{LoopFrames: 120}` still set) | its block and the pooled block: `Available false`, `Ms nil`, the reason above with the counts; `"ms":` occurs nowhere in the JSON (the synthetic viewers' packet transit is unavailable too, so any `"ms":` would be a one-way delay number) |
| `TestOneWayDelayWithoutASendLog` | `Command: "view"`, no `Fingerprint`, `viewers(3, 0)` | every viewer block and the pooled block unavailable, reason contains ``the send log lives in the `run` process``; `PacketTransit` blocks as before |
| `TestOneWayDelayNoTrack` | `Fingerprint{LoopFrames: 120}`, `viewers(3, 0)` (no blocks) | each viewer's reason `NoTrackReason`; pooled reason `NoTrackReason` |
| `TestPacketTransitKeys` (**rewritten**, D8) | the builds of `TestOneWayDelayPools`, `TestOneWayDelayWithoutASendLog` and `TestOneWayDelayNoTrack` | in each JSON `strings.Count(strings.ToLower(js), "one-way delay") == 1`, and `strings.Count(strings.ToLower(OneWayDelayMethod), "one-way delay") == 1` — so the one occurrence is that line, never a reason or packet transit's; `"packetTransit": {` count 4; `latency` absent in any case |
| `TestOneWayDelayMethodLine` | `OneWayDelayMethod`, `Method` | contains `first-packet send to last-packet arrival`, `incomplete 1 s after its first packet`, `pending when it stops are not counted`, `a viewer's first frame`, `is not measured`; it is in `Method` at an index lower than the line starting `"Packet transit:"` |
| `TestMarkdownLeadsWithOneWayDelay` | the `TestOneWayDelayPools` build; then the `TestOneWayDelayWithoutASendLog` build | first: `"| one-way delay (fingerprint, per frame) |"` precedes `"| join: first keyframe |"`; the viewers header contains `"| delay p50 | delay p99 | transit p50 |"`. second: `"**One-way delay: unavailable** — no send log"` precedes `"**Packet transit: unavailable**"` |

`cmd/whipbench/main_test.go` (imports `internal/report` and `internal/stats`):

| Case | Input | Expected |
|---|---|---|
| `TestHeadline` (available) | `report.Aggregate{OneWayDelay: []report.OneWayDelay{{Source: "fingerprint", Available: true, Ms: &stats.Summary{P50: 12.3, P99: 45.6}}}, PacketTransit: report.PacketTransit{Reason: "not negotiated"}}` | exactly `"one-way delay (fingerprint) p50 12.3 ms, p99 45.6 ms\njoin (first keyframe) p50 0 ms, p95 0 ms; loss 0.000%; packet transit unavailable: not negotiated\n"` |
| `TestHeadline` (unavailable) | the same with the block `{Source: "fingerprint", Reason: report.NoSendLogReason}` | starts with `"one-way delay unavailable: no send log"`; "packet transit" appears after it |

`internal/metrics/metrics_test.go`: `TestExposition` unchanged in behaviour.

The pooling, reasons, block count and the D8 count were checked on a prototype of this
step against `74a7ad2` (2026-10-03), as were the end-to-end figures quoted in S6 and S7.

### Verify

```sh
go test -race -count=1 ./internal/report/ ./internal/metrics/ ./cmd/whipbench/ -v; echo "exit $?"
go build ./... ; echo "exit $?"
grep -c 'first-packet send to last-packet arrival' internal/report/report.go   # prints 1
git grep -n -i 'one-way delay' -- internal/report/report.go | grep -v 'OneWayDelayMethod =' | grep -v '^\S*:[0-9]*:\s*//'   # prints nothing (the last grep exits 1)
```

The last command allows "one-way delay" in Go comments and in the `OneWayDelayMethod`
line only — strings elsewhere in `report.go` would reach the JSON. It was run with macOS's
BSD grep against a prototype (`\s`, `\S` work there and in GNU grep).

### Acceptance criteria

- [ ] Every test case above passes, and the shared gate exits 0
- [ ] `TestRunWritesReports` still passes unchanged (no new assertion; until S6 its report says one-way delay is unavailable with `NoSendLogReason`)
- [ ] `viewer.go` differs from `origin/main` by the one `Result` field only

---

## S6 · Wire it into `run`

Starts from an `origin/main` that has S1, S2, S3 and S5.

### Files touched

| Path | Action |
|---|---|
| `internal/publisher/publisher.go` | modify |
| `internal/viewer/viewer.go` | modify |
| `internal/runner/runner.go` | modify |
| `internal/runner/runner_test.go` | modify |

### Changes

#### `internal/publisher/publisher.go`

1. `Config` (`:32-45`), after `Live` (`:39`): `SendLog *fingerprint.SendLog` with the
   comment "logs t0 per frame for one-way delay (WB-38); nil in `publish`, which keeps no
   log". Import `internal/fingerprint`.
2. `Stream`, inside `for i, pl := range payloads` (`:247`), immediately **before** `if err
   := p.track.WriteRTP(pkt); err != nil` (`:257`), after the stamp (`:253-256`): `if i == 0
   { p.cfg.SendLog.Record(k, time.Now()) }` — t0 is logged before the first `WriteRTP`, so
   no viewer can hold a frame whose send is not logged (D2). `Record` on a nil log is a
   no-op (S2). Update the `Stream` doc comment (`:195-201`) with one sentence saying so.

#### `internal/runner/runner.go`

1. After the clip check (`:52-54`), before `live` (`:66`): declare `var table
   *fingerprint.Table; var sendLog *fingerprint.SendLog`; when `sc.WHIP != ""`, `table,
   err = fingerprint.NewTable(opt.Clip)` (error → `return nil, fmt.Errorf("runner: %w",
   err)`) and `sendLog = fingerprint.NewSendLog(table.Frames())`; both stay nil otherwise.
2. After `in := report.Input{…}` (`:75`): when `table != nil`, `in.Fingerprint =
   &report.Fingerprint{DuplicateFrames: table.DuplicateFrames(), LoopFrames:
   table.Frames()}` — set before the publisher connects, so the early-return report
   (`:85-91`) carries it too.
3. `publisher.Config` (`:82-84`): add `SendLog: sendLog`.
4. `viewer.Config` (`:132-135`): add `Frames: table, SendLog: sendLog`.
5. After `in.Interrupted = …` (`:151`), before `report.Build` (`:152`): when
   `in.Fingerprint != nil` and `sendLog.LoopMin()` is ok, `in.Fingerprint.LoopMin = d`. The
   publisher has stopped by then (`:145-149`).

#### `internal/viewer/viewer.go`

1. `Config` (`:41-52`), after `Live`: `Frames *fingerprint.Table` and `SendLog
   *fingerprint.SendLog`, with the comment "both set by run, from the clip it publishes,
   turn on one-way delay; view leaves them nil and the report says why". Imports
   `internal/fingerprint`, `internal/reassembler`.
2. `progress` (`:105-123`): add `owdOn bool` and `owd OneWayDelay`, guarded by `pr.mu`
   like the rest.
3. `OnTrack` (`:217-233`), under the existing lock, after `pr.transit.hist = …` (`:228`):
   when `cfg.Frames != nil && cfg.SendLog != nil`, `pr.owdOn = true`, `pr.owd =
   OneWayDelay{Source: SourceFingerprint, Hist: stats.NewHistogram()}`. The call at `:232`
   becomes `readLoop(track, pr, live, cfg.Frames, cfg.SendLog)`.
4. `readLoop` becomes `func readLoop(track *webrtc.TrackRemote, pr *progress, live
   *metrics.Live, frames *fingerprint.Table, log *fingerprint.SendLog)`. In its opening
   lock (`:303-305`), which already reads the local `codec` (`"vp8"`/`"h264"`, from
   `rtc.CodecName` at `:224` — the same strings as `clip.VP8`/`clip.H264`), declare `var rs
   *reassembler.Reassembler; var m *fingerprint.Matcher` before the lock and, when
   `pr.owdOn`: `rs, err = reassembler.New(codec)`; on error set
   `pr.owd.Reason = fmt.Sprintf("codec %q has no fingerprint", codec)` and leave `rs` nil;
   else `m := fingerprint.NewMatcher(log, frames.Ticks())`. Keep `lastInc uint64`.
5. In the loop, **after** `pr.mu.Unlock()` (`:343`) — hashing stays outside the lock
   (Structure risk S6): when `rs != nil`, `done := rs.Push(pkt, now)` (`now` from `:312`,
   monotonic); if `len(done) == 0 && rs.Incomplete() == lastInc`, continue. Otherwise
   classify every frame of `done` (below) without the lock, then lock once: per frame
   `CompleteFrames++` and `sampled` → `pr.owd.Hist.Add(ms)`, `invalid` → `Invalid++`,
   `unmatched` → `UnmatchedFrames++`, `duplicate` → nothing more; then
   `IncompleteFrames = rs.Incomplete()`, `FrameEnd = rs.FrameEnd()`; unlock;
   `lastInc = rs.Incomplete()`.

`outcome`, its constants and `classify` go at the end of `viewer.go`; none of the four
names exists in package `viewer` today.

```go
// outcome is what one complete frame contributes to the viewer's block.
type outcome int

const (
	sampled outcome = iota
	invalid   // aliased or not logged (P6)
	unmatched // rejected, nothing to hash, or not in the clip
	duplicate // a clip duplicate: complete, never sampled
)

// classify hashes and matches one complete frame. It runs outside pr.mu.
func classify(f reassembler.Frame, codec string, frames *fingerprint.Table, m *fingerprint.Matcher) (outcome, time.Duration)
```

   `classify`: `f.Rejected` → `unmatched`; `fingerprint.Of(codec, f.Payload)` not ok →
   `unmatched`; `frames.Lookup`: `Unknown` → `unmatched`, `Duplicate` → `duplicate`,
   `Unique` → `m.Match(i, f.Timestamp, f.Arrival)`: `Sampled` → `(sampled, d)`, else
   `invalid`.
6. Finaliser (`:183-198`), after `res.PacketTransit = …`: `if pr.owdOn { res.OneWayDelay =
   []OneWayDelay{pr.owd} }`. The report finishes it (S5).

**Do NOT touch:** the packet-transit code in `readLoop` (`:330-342`), the join logic
(`:321-329`), `internal/report/*` (S5 owns it).

### Tests (`internal/runner/runner_test.go`)

| Case | Input | Expected |
|---|---|---|
| `TestRoundTripVP8` (extended, after its current checks) | as today: VP8, 4 viewers | `p := rep.Aggregate.FingerprintDelay()`: `Available`, `Viewers == 4`, `ViewersByFrameEnd["marker"] == 4`, `Ms != nil`, `Ms.P50 > 0`, `Ms.P99 < 1000`, `Invalid == 0`, `UnmatchedFrames == 0`, `LoopFrames == 120`, `DuplicateFrames == 0`, `LoopMinMs != nil && *LoopMinMs > 3000`. Each viewer: one block, `Source "fingerprint"`, `Available`, `FrameEnd "marker"`, `Samples > 0`, `Invalid == 0`, `UnmatchedFrames == 0` |
| `TestRoundTripH264` (extended) | as today: H.264, 2 viewers | the same per-viewer assertions; pooled `Available`, `Viewers == 2` |
| `TestRoundTripPublisherWithoutASendLog` (new, `t.Parallel()`) | a relay `testserver.New(testserver.Options{RTC: loop})` behind `httptest.NewServer`; the embedded VP8 clip; `publisher.Connect(ctx, publisher.Config{WHIP: hs.URL + "/whip", Clip: c, RTC: loop})`; `Stream` for 1 s | `FramesSent >= 20`, `Error == ""` — `publish`'s path, no log |
| `TestRoundTripPublisherFillsTheSendLog` (new, `t.Parallel()`) | the same with `SendLog: fingerprint.NewSendLog(120)` | `log.Recorded() == res.FramesSent`, `FramesSent >= 20`; `log.Latest(0, time.Now())` is ok |
| `TestMethodStatesTheReassemblyWindow` (new) | — | `reassembler.Window == time.Second` and `report.OneWayDelayMethod` contains `incomplete 1 s after its first packet` |

Share the relay-and-publisher setup of the two publisher tests in one helper
`publishFor(t, log *fingerprint.SendLog) publisher.Result`: `srv, err :=
testserver.New(testserver.Options{RTC: loop})` (`loop` is `runner_test.go:32`), `hs :=
httptest.NewServer(srv)`, `t.Cleanup(func() { hs.Close(); srv.Close() })`; `c, err :=
clip.Load("vp8", whipbench.ClipVP8)`; `ctx` with a 10 s timeout; `pub, err :=
publisher.Connect(ctx, publisher.Config{WHIP: hs.URL + "/whip", Clip: c, RTC: loop,
SendLog: log})` (`publisher.go:86`); `sctx` = `ctx` with a 1 s timeout; `return
pub.Stream(sctx)` — `Stream` ends the session itself (`:273`). `FramesSent` and
`Recorded()` are both `uint64`. They are equal by construction: `ctx.Done` is checked
before a frame starts (`:236`), a failed `WriteRTP` only `continue`s (`:258`), and
`FramesSent++` (`:266`) follows every frame whose first packet was logged; a frame with
no payloads is the one exception, and the clips have none. Prototype figures at
`74a7ad2`, for orientation only: per viewer 97-119 samples, p50 ≈ 0.5 ms, `loopMinMs` ≈
3999.

**Split if the session passes 40% (P9)** — branches `wb-38/s6a`, `wb-38/s6b`: S6a = `publisher.go`, `runner.go` items 1-3 and 5,
the three new tests; verify `-run 'TestRoundTripPublisher|TestMethodStates'` (reports then
say `no video track reached this viewer`, a reason, not a number). S6b = `viewer.go`,
`runner.go` item 4, the two extended round trips; verify with the full command below.

### Verify

```sh
go test -race -count=1 ./internal/runner/ -run 'TestRoundTrip|TestMethodStates' -v; echo "exit $?"
```

### Acceptance criteria

- [ ] Every test case above passes, and the shared gate exits 0 — `TestRunWritesReports` and every other runner test unchanged and green
- [ ] No hashing or reassembly happens while `pr.mu` is held

---

## S7 · Markerless and lossy relays

Starts from an `origin/main` that has S6.

### Files touched

| Path | Action |
|---|---|
| `internal/testserver/testserver.go` | modify |
| `internal/runner/runner_test.go` | modify |
| `thoughts/WB-38-frame-fingerprint/99-progress.md` | modify — the observation and a Discovery |

### Changes

#### `internal/testserver/testserver.go`

1. `Options` (`:31-41`), after `DropEvery`: `ClearMarker bool` — "forwards every packet
   with the marker bit cleared, like a server that does not keep it (D5)". Add "one that
   clears the marker" to the package comment's list (`:9-11`).
2. `forward` (`:215-217`): `Marker: pkt.Marker && !s.opt.ClearMarker`.

#### `internal/runner/runner_test.go`

Two tests; both names match the Verify pattern.

```go
func TestMarkerlessRelay(t *testing.T)          // new
func TestLossThroughALossyRelay(t *testing.T)   // extended (:194-205)
```

1. `TestMarkerlessRelay` (`t.Parallel()`): `rep := run(t, testserver.Options{ClearMarker:
   true}, base("vp8", 2), "")`. For **every** viewer (joined or not): one block, `FrameEnd
   == "timestamp"`, `Available`, `Samples > 0`, `Invalid == 0`, `UnmatchedFrames == 0`.
   Do **not** assert `Joined`, the verdict or the pooled block (P8): without a marker no
   keyframe completes (`internal/rtpstats/rtpstats.go:248-252`), so no viewer joins and
   the pooled block says `no viewer joined`. A comment above the test says so.
2. `TestLossThroughALossyRelay`: keep the loop and its loss assertion; for each **joined**
   viewer add: `b := v.OneWayDelay[0]`; `b.IncompleteFrames > 0`, `b.Available`,
   `b.Samples > 0`, `b.Samples <= b.CompleteFrames`, `b.UnmatchedFrames == 0` — an
   incomplete frame that reached a hash would almost surely miss the table, so zero
   unmatched shows none did. Then `t.Logf("viewer %d: received %d, lost %d, about %d
   dropped by the relay", v.ID, v.RTP.Received, v.RTP.Lost, (v.RTP.Received+v.RTP.Lost)/50)`.

#### `thoughts/WB-38-frame-fingerprint/99-progress.md`

Run `go test -race -count=1 ./internal/runner/ -run TestLossThroughALossyRelay -v` and read
the `t.Logf` lines. `lost` within one of `about … dropped` for every viewer → holes (the
relay's NACK responder never saw the dropped packets, `testserver.go:210-212`); clearly
fewer → retransmissions. Record in **Discoveries**:

| # | Discovery | Path | Action |
|---|---|---|---|
| n | `DropEvery: <YYYY-MM-DD> holes` (or `retransmissions`) `— lost <x> of ≈<y> dropped per viewer; whether 1 s outlasts pion's NACK retries is <not observable on this relay / observed: …>` | `internal/testserver/testserver.go:210-212` | ignore (D4's open item stays open for WB-41) |
| n+1 | `A markerless stream never joins: rtpstats completes a keyframe on its marker only, so through ClearMarker every viewer ends with errorKind run_ended (internal/viewer/viewer.go:282-283) and the run has no verdict` | `internal/rtpstats/rtpstats.go:248-252` | follow-up |

The prototype at `74a7ad2` saw holes: lost 4-7 of ≈4-7 dropped per viewer, both codecs, three runs.

### Tests

| Case | Input | Expected |
|---|---|---|
| `TestMarkerlessRelay` | VP8, 2 viewers, `ClearMarker` | every viewer: `frameEnd "timestamp"`, available, samples > 0 (prototype: 103-118), invalid 0, unmatched 0 |
| `TestLossThroughALossyRelay` | VP8, 2 viewers, `DropEvery: 50` | loss 1-3% (unchanged); each joined viewer: incomplete > 0 (prototype: 5-6), available, samples ≤ complete, unmatched 0 |

### Verify

```sh
go test -race -count=1 ./internal/runner/ -run 'TestMarkerless|TestLoss' -v; echo "exit $?"
grep -nE 'DropEvery: [0-9]{4}-[0-9]{2}-[0-9]{2} (holes|retransmissions)' thoughts/WB-38-frame-fingerprint/99-progress.md   # one line
```

### Acceptance criteria

- [ ] Both tests pass; the shared gate exits 0
- [ ] The two Discoveries are in `99-progress.md`, the first dated and naming holes or retransmissions

---

## S8 · Docs, CHANGELOG and backlog

Starts from an `origin/main` that has S5, S6 and S7 (and H1's answer, if it came).

### Files touched

| Path | Action |
|---|---|
| `README.md` | modify |
| `CHANGELOG.md` | modify |
| `BACKLOG.md` | modify |
| `ROADMAP.md` | regenerated by `npm run roadmap`, never edited by hand |
| `CLAUDE.md` | modify (P10) |

### Changes

#### `README.md`

1. `:5` (intro): put "one-way delay" first in the list of what the report covers.
2. `:7` (Status): the last sentence says the method is decided **and built** — `run`
   reports one-way delay per frame by frame fingerprint ([WB-38](BACKLOG.md)) — and that
   the first live figure is [WB-5](BACKLOG.md)'s.
3. Definitions table (`:45-54`): a new row **above** `packet transit` (`:53`), `| one-way
   delay | … |`, whose cell contains the exact phrase `first-packet send to last-packet
   arrival` (P1) and says: per frame; the publisher logs when it hands the frame's first
   packet to the stack, the viewer reassembles and hashes the frame (SHA-256 of the VP8
   frame, or of the H.264 VCL NAL units — types 1-5, pending a check against H.264 Table
   7-1) and subtracts that send time from its last packet's arrival; a match proven whole
   loops too new, or with no logged send, is invalid; one sample per
   complete frame, on the one process's monotonic clock; `run` only — `view` reports it
   unavailable. Keep the `packet transit` row as it is.
4. `:56` (percentiles): one-way delay and packet transit both pool every sample of every
   viewer that has one, in a histogram with 1% buckets.
5. `:73`: the paragraph's lead becomes **One-way delay — the headline, built in `run`
   ([WB-38](BACKLOG.md)).** Keep its explanation of the two sources and the pointers to
   WB-39, WB-40, WB-41, WB-3. Add: the report key `oneWayDelay` is a list of source blocks,
   per viewer and pooled (`source`, `frameEnd`, `completeFrames`, `incompleteFrames`,
   `samples`, `invalid`, `unmatchedFrames`; pooled adds `viewers`, `viewersByFrameEnd`,
   `duplicateFrames`, `loopFrames`, `loopMinMs`); a frame incomplete 1 s after its first
   packet is counted and never hashed; a viewer's first frame and frames pending at stop
   are not counted; a frame later than `loopMinMs` is caught only by the RTP timestamp
   check, and a viewer's first match is taken as it is; no Prometheus series yet
   (WB-42); the CPU cost per viewer is unmeasured (WB-25).
6. Limits list (`:128-132`): a bullet after the packet-transit one (`:131`): **One-way
   delay needs the publisher in the same process** (`run`), and the cost of hashing every
   frame on every viewer is not yet measured.

`scripts/check-repo.sh:15-24` greps README for `not glass-to-glass`, `packet transit:
unavailable`, `No-verdict rule`, `Hosts only`, the load-testing sentence, the
`docs/load-testing-etiquette.md` link and `more than 10% of the viewers` — keep every one.

#### `CHANGELOG.md`

Under `## [Unreleased]` → `### Added` (`:8`), a new **first** bullet: one-way delay per
frame by frame fingerprint (WB-38) in `run`; the new report key `oneWayDelay` (a list of
source blocks, source `fingerprint`, per viewer and in the aggregate); the schema stays
`whipbench.report/v0` (a key added, none changed meaning); the Markdown table and the stdout
line lead with it, packet transit unchanged below; `view` reports it unavailable; and the
two departures that change what a reader gets: whipbench's own reassembler with a 1 s
window instead of pion's samplebuilder (D4), and no retransmission count beside the figure
until WB-41 (D6). No Prometheus series (WB-42).

#### `BACKLOG.md`

1. **WB-38** (`:96-111`): `- [ ]` → `- [x]`; before its meta comment append `Done
   <YYYY-MM-DD>: QRSPI in thoughts/WB-38-frame-fingerprint/ (#<first>–#<last>); two
   departures from this text, approved in the Design — whipbench's own reassembler instead
   of samplebuilder (D4), retransmissions beside the figure only with WB-41 (D6) — and the
   loop is counted in frames, not seconds (D3).`, wrapped and indented like the item; the
   meta comment becomes `<!-- wb: prio=high size=L labels=measurement,client ver=main -->`.
2. Three new items at the end of the v0.2.0 milestone, after WB-41 (`:164`), before
   `## v0.3.0` (`:166`), the highest id being WB-41:

```
- [ ] **WB-42 — Live one-way delay series with a source label**: a Prometheus histogram of
  WB-38's per-frame figure, `whipbench_one_way_delay_seconds{source="fingerprint"}`, so a
  long run shows the headline while it runs. WB-38 opened none (its D8): an unlabelled
  series would change meaning when WB-39's `stamp` source lands, and
  `internal/metrics/metrics_test.go` keeps the name shut until the label exists.
  <!-- wb: prio=med size=S labels=report -->
- [ ] **WB-43 — Frame rate for Annex-B clips from the clip**: `clip.Load` reads H.264 at a
  hard-coded 30 fps (`internal/clip/clip.go:231`), so a clip at another rate is paced and
  timestamped wrong; take the rate from the SPS timing information or a flag. One-way
  delay is safe either way — it counts the loop in frames (WB-38, D3) — the pacing is not.
  <!-- wb: prio=low size=S labels=client -->
- [ ] **WB-44 — One-way delay split by keyframe and delta frame**: a keyframe spans many
  packets and a delta frame few, so their first-to-last spread differs; report both
  distributions beside the pooled figure, as WB-38's Questions phase deferred (Q3).
  <!-- wb: prio=low size=M labels=measurement,report -->
```

3. `npm run roadmap`, then commit the regenerated `ROADMAP.md`.

The PR numbers for WB-38's Done line: `gh pr list --state merged --search 'WB-38 in:title'
--json number,title` — the lowest and highest of the WB-38 pull requests, the QRSPI phases
included.

#### `CLAUDE.md` (P10)

Layout (`:15-38`): after the `internal/viewer/` line add `internal/fingerprint/    frame
fingerprints, the clip's table, the send log and the match (WB-38)` and
`internal/reassembler/    per-viewer frame reassembly: packets and arrivals in, complete
frames out`; the `internal/viewer/` line (`:22`) says "one-way delay and packet transit or
why not". Rule 5 (`:55-57`): one-way delay is WB-38, built, in `run` only;
capture-to-decode is WB-2, not implemented yet. Align with spaces as the block does.

### Tests

None new: S8 changes no Go code. The checks below are the tests.

### Verify

```sh
sh scripts/check-repo.sh; echo "exit $?"
npm run roadmap && npm run backlog; echo "exit $?"
git status --porcelain ROADMAP.md                                    # empty after the commit
grep -l 'first-packet send to last-packet arrival' README.md internal/report/report.go internal/fingerprint/fingerprint.go | wc -l | tr -d ' '   # prints 3
grep -c '^- \[x\] \*\*WB-38 — ' BACKLOG.md                           # prints 1
awk '/^```/{f=!f;next} !f' BACKLOG.md | grep -cE '^- \[ \] \*\*WB-4[234] — '   # prints 3 (the fenced WB-99 example excluded)
awk '/^## \[Unreleased\]/{f=1;next} /^## \[/{f=0} f' CHANGELOG.md | grep -c 'oneWayDelay'   # prints 1 or more
grep -c 'neither implemented yet' CLAUDE.md                          # prints 0
```

`npm run backlog` needs the network (`npx backlogsync@0.1.0`); CI's backlog workflow
repeats it.

### Acceptance criteria

- [ ] Every command above gives its stated result; the shared gate exits 0
- [ ] `node scripts/leakcheck.mjs` exits 0 before the push

---

## H1 · H.264 VCL NAL types against Table 7-1 (the maintainer's)

Not for an agent. With H.264 (ITU-T Rec. H.264) Table 7-1 in hand: for the NAL unit types
a base-profile stream such as the clips can carry (Annex A), are the VCL types exactly
1-5? Types 20 and 21 are VCL only under the SVC and MVC annexes (Annexes G, H); say whether
they belong in `isVCL`. Then tick the box in `thoughts/WB-38-frame-fingerprint/02-design.md`,
More research needed, as `- [x] D1: H.264 VCL NAL types are exactly 1-5 — verified
<date>, Table 7-1 (<edition>)`, or with the corrected set.

- **The answer is 1-5:** nothing else changes.
- **It is not:** on branch `wb-38/h1`, its own PR, before S8 if it can (else S8's README
  cell is fixed in the same PR), one line changes — the body of `isVCL` in
  `internal/fingerprint/fingerprint.go` (S1) — plus one row in `TestFingerprintIsVCL` for
  the type that moved, and S8's README cell states the corrected set. Nothing else names
  the types (P2).

```sh
grep -n '\[x\] D1: H.264 VCL NAL types' thoughts/WB-38-frame-fingerprint/02-design.md   # the ticked, dated line
```

---

## Rollback

Every step is additive; nothing migrates and nothing persists outside a report file.

- **One step:** `git revert <its squash commit>` on `main`, latest first (S8 → S1). The
  repo is working after each revert: S6's revert leaves reports saying one-way delay is
  unavailable with a reason (S5's state); S5's revert removes the key, back to the v0
  report of today.
- **The figure is wrong after release:** revert S6 alone — no fake number ships, the key
  stays and says unavailable — and reopen WB-38 in `BACKLOG.md` (`- [ ]`, drop `ver=`).
- **Reports already written** keep their `oneWayDelay` blocks; the schema is v0 either way,
  so no consumer breaks on its presence or absence.
- No Prometheus series, flag or scenario key is added, so none needs retiring.

---

## Status

- [x] Every step has exact paths
- [x] Every new function has a complete signature
- [x] Every test case has inputs and expected outputs
- [x] Every verification command is copy-pasteable
- [x] **Zero-context test:** an agent reading only this file can execute it
- [x] Rollback plan present
- [x] Approved (2026-10-03, Allan Nava, in chat: "approvo")

> Next phase: **Implement**. It receives: this file + `99-progress.md`.
> One session per step.
