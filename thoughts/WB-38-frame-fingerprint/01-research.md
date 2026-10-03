# 01 · Research — WB-38 One-way delay by frame fingerprint

**Written against:** `5e86468`

This is the most expensive phase and the one with the highest compression ratio
(~30-50×). It is also the one that most needs subagents.

All paths are repo-root-relative. Pion versions are from `go.mod:6-9`: `pion/webrtc/v4`
v4.2.22, `pion/rtp` v1.10.5, `pion/interceptor` v0.1.49. Pion paths below are written as
`rtp@v1.10.5/...` and `webrtc/v4@v4.2.22/...`, meaning that module in the Go module cache.

---

## Reference questions

From `00-questions.md` — every default was accepted on 2026-10-03:

- Q1 → a sample is invalid only when the viewer has its own evidence that the frame is more than one loop old; otherwise it is kept, and the report states that delays of a loop or more alias modulo the loop length. The loop length is read from the loaded clip, not hard-coded.
- Q2 → single-process `run` only, with one monotonic clock and one in-memory log. A split run reports the fingerprint delay as unavailable, with the reason.
- Q3 → one distribution over all complete frames, from first-packet send to marker arrival. The definition is printed next to the figure, and keyframes are not split out.
- Q4 → t1 is the marker packet's arrival, original or retransmitted. Frames that needed a retransmission are counted beside the figure. A frame is incomplete when the reassembler gives it up, after a fixed window stated in the report.
- Q5 → the figure always carries its sample count and `unmatchedFrames`. With zero matches it is unavailable, with the reason. There is no minimum match fraction. `unmatchedFrames` counts only complete frames whose hash is absent from the log.
- Q6 → `oneWayDelay` is the fingerprint figure, with `source: fingerprint`. Packet transit stays under its own key, unchanged and never the headline. Old reports are not rewritten.
- Q7 → every hash that occurs more than once in one loop is excluded from sampling. Those frames are still sent, and their count is reported as a fixed property of the clip.
- Q8 → the H.264 hash covers the VCL NAL units in order, each with its one-byte header and without start codes or length prefixes. SPS, PPS, SEI and AUD are skipped.
- Q9 → without a marker, t1 is the recorded arrival of the last packet carrying the frame's timestamp, read once the next timestamp arrives. The fallback is per stream, and the report records which rule each viewer used.
- Q10 → always on in `run`, with no flag. The report notes that the CPU cost is unmeasured (WB-25).

---

## Map of the territory

### Components involved

| Area | Path | Role |
|---|---|---|
| Embedded clips | `clips.go:12-19` | `ClipVP8` and `ClipH264` are embedded with `go:embed` from `testdata/clip-vp8.ivf` and `testdata/clip-h264.h264` |
| Clip model | `internal/clip/clip.go` | `Frame{Data, Key}` and `Clip{Codec, Ticks, Frames}`. Parses IVF and Annex-B, loops frames, derives RTP timestamps |
| Publisher | `internal/publisher/publisher.go` | WHIP client. Packetises each frame itself and writes RTP to a `TrackLocalStaticRTP` |
| Viewer | `internal/viewer/viewer.go` | WHEP client. One `ReadRTP` loop per viewer, per-packet stats and packet transit |
| RTP stats | `internal/rtpstats/rtpstats.go` | Per-stream loss, jitter, stalls and keyframe tracking from packet headers. Pure arithmetic, no network |
| Pion setup | `internal/rtc/rtc.go` | `NewAPI`: codecs, RTCP feedback, the abs-capture-time extension, default interceptors |
| Orchestration | `internal/runner/runner.go` | `Run`: publisher and every viewer in one process, sharing one `*metrics.Live` |
| Report | `internal/report/report.go`, `internal/report/markdown.go` | JSON schema `whipbench.report/v0`, aggregate, verdict, Method lines, Markdown rendering |
| Statistics | `internal/stats/stats.go` | Exact nearest-rank `Summarise` and a fixed-memory log-bucket `Histogram` |
| Live metrics | `internal/metrics/metrics.go` | Atomic counters and a hand-written Prometheus exposition |
| CLI | `cmd/whipbench/main.go` | Subcommands `publish`, `view`, `run`, `version`, `help` |
| Test relay | `internal/testserver/testserver.go` | In-process pion WHIP/WHEP relay (one publisher, many viewers) for the tests |
| Clip generation | `scripts/make-clips.sh` | ffmpeg `testsrc2`, 640x360, 30 fps, 4 s, GOP 30, VP8 (IVF) and H.264 (Annex-B) |
| Eval | `evals/2026-10-02-mediamtx-fingerprint.md` | A throwaway probe: MediaMTX v1.21.1 against reassembled-frame hashes |

### Entry points

| Symbol | Path:line | What it does |
|---|---|---|
| `loadClip` | `cmd/whipbench/main.go:150-159` | Takes the embedded bytes, or the `--clip <path>` file, then calls `clip.Load` |
| `clip.Load` | `internal/clip/clip.go:226` | `vp8` → `ParseIVF(data)`. `h264` → `ParseH264(data, 30)`, with 30 fps hard-coded at `:231` |
| `ParseIVF` | `internal/clip/clip.go:106` | Checks the DKIF/VP80 header. A frame is the raw IVF payload, sliced without a copy. Ticks come from the IVF rate, scale and pts step (`:140-151`) |
| `ParseH264` | `internal/clip/clip.go:159` | `SplitAnnexB`, then groups NALs into access units and re-prefixes **every** NAL with `00 00 00 01` (`:192-193`). No NAL type is dropped |
| `SplitAnnexB` | `internal/clip/clip.go:200` | Splits on 3- and 4-byte start codes, returns NAL units without them, and trims trailing zeros |
| `Clip.Frame(k)` / `Clip.Timestamp(base,k)` | `internal/clip/clip.go:53`, `:63` | `Frames[k % len]`; `base + k·Ticks` as uint32. k counts across loops, so the timestamp keeps rising across a loop wrap |
| `Clip.Duration` / `FrameDuration` | `internal/clip/clip.go:48`, `:43` | `len(Frames)·Ticks/90000` and `Ticks/90000`. Computed, not hard-coded |
| `publisher.Connect` | `internal/publisher/publisher.go:86` | The only constructor. Runs the WHIP exchange and waits for Connected. `Config` is at `:33-45` |
| `(*Publisher).Stream` | `internal/publisher/publisher.go:202` | The send loop. Blocks and returns `Result`. No callback, channel or per-frame hook |
| send loop body | `internal/publisher/publisher.go:244-269` | Packetises frame k (MTU 1200, `:30`), sets the marker on the last payload (`:249`), stamps abs-capture-time per packet with `time.Now()` (`:254`), then `WriteRTP` (`:257`) |
| `viewer.Run` | `internal/viewer/viewer.go:132` | Builds its own pion API and recvonly transceiver. `OnTrack` at `:217` accepts only the first track. The join origin is `time.Now()` at `:251` |
| `readLoop` | `internal/viewer/viewer.go:302` | `track.ReadRTP()` at `:308` (interceptor attributes discarded). `now := time.Now()` at `:312`. Under `pr.mu`, `rtpstats.Stream.Add`, then packet transit |
| `finishPacketTransit` | `internal/viewer/viewer.go:347` | Turns per-viewer transit counts into available, or unavailable with a reason string |
| `runner.Run` | `internal/runner/runner.go:46` | `live := &metrics.Live{}` (`:66`), `publisher.Connect` (`:82`), one goroutine per viewer calling `viewer.Run` (`:132`) |
| `report.Build` / `aggregate` | `internal/report/report.go:132`, `:173` | Assembles the report. Pools per-viewer transit histograms with `Histogram.Merge` |
| `cmdPublish` / `cmdView` / `cmdRun` / `execute` | `cmd/whipbench/main.go:168`, `:221`, `:249`, `:286` | `publish` writes no report. `view` and `run` go through `execute`, which writes JSON + Markdown and prints a stdout headline |

---

## Existing patterns and conventions

### Unavailable, never estimated

- **Where:** `internal/report/report.go:110-115` (aggregate `PacketTransit`: `available`, `reason`, `viewers`, `ms`) and `internal/viewer/viewer.go:54-68` (per viewer: `available`, `reason`, `negotiated`, `stamped`, `invalid`, `ms`, plus an unexported histogram).
- **How it works:** a missing input gives `available:false` and a `reason` string, and `ms` (a `*stats.Summary`) is nil and omitted. It is never 0 or null. The aggregate reason is "no viewer joined" or the most common per-viewer reason (`mostCommon`, `report.go:256`). When only some viewers had samples, `reason` says "pooled over X of Y joined viewers…".
- **Who already uses it:** `internal/report/markdown.go:68-72` prints "**Packet transit: unavailable** — <reason>." The stdout line is at `cmd/whipbench/main.go:304-311`. The rule is stated in `CLAUDE.md:46-48`, `CONTRIBUTING.md:25-27` (which names "clocks that disagree") and `README.md:70`.

### Plausibility gate turning a figure unavailable

- **Where:** `internal/viewer/viewer.go:35` (`MaxPlausibleTransit` = 60 s) and `:39` (`MaxInvalidStampShare` = 0.01).
- **How it works:** a sample below 0 or above 60 s counts as `invalid` and is not recorded. If more than 1% of the stamped packets are invalid, the viewer's figure is unavailable, with the counts in the reason.
- **Who already uses it:** packet transit only.

### Definition written in three places

- **Where:** `CLAUDE.md:44-45`, `CONTRIBUTING.md:17-24`.
- **How it works:** a measurement is defined in the README table, in the `Method` lines (`internal/report/report.go:289`, one string per figure, appended to every report) and in the package comment, all saying the same thing. It is tested on synthetic input before any network test. Definitions render only as a list at the end of the Markdown (`markdown.go:124-127`), not next to the figure.
- **Who already uses it:** packet transit (`report.go:294`) and ramp offsets (`RampOffsetMethod`, `report.go:300`, appended only when the feature is on, `:150`).

### Report compatibility

- **Where:** `internal/report/report.go:31-32` (`Schema`, "bump it when a field changes meaning"), `CHANGELOG.md:24-30` (`latency` renamed to `packetTransit` with the schema kept at v0, since no key changed meaning, and no alias), `CHANGELOG.md:9-15` (WB-8 added `rampOffsetMs`, with the report unchanged when the feature is off).
- **Who already uses it:** `evals/2026-10-01-mediamtx-local/*.json` still carry the old `latency` key under schema v0 and were not rewritten.

### Per-viewer histogram, pooled at report time

- **Where:** `internal/stats/stats.go:62-150`. 1% log buckets from 1 µs to 60 s, about 1,800 uint64 buckets (fixed memory), exact min/max/sum/n, quantiles within 0.5%, exact `Merge` (`:111`). Not safe for concurrent use.
- **How it works:** each viewer fills its own histogram under `pr.mu`, and `report.aggregate` merges them. Per-viewer value sets (one value per viewer) use exact `Summarise` (`:38`). JSON percentiles are the flat keys `n`, `min`, `mean`, `p50`, `p95`, `p99`, `max` (`stats.go:15-23`).
- **Who already uses it:** packet transit. Live metrics mirror it as `whipbench_packet_transit_seconds` (`internal/metrics/metrics.go:19`, `:51`).

### Frame boundaries from RTP headers, without reassembly

- **Where:** `internal/rtpstats/rtpstats.go:218-262` (`keyframes`, `keyframes.add`) and `:264-330` (`KeyframeStart`, `vp8KeyStart`, `h264KeyStart`).
- **How it works:** a keyframe is complete when the marker packet arrives with the pending keyframe's timestamp and the packet count equals seq(marker) − seq(start) + 1. The start is found from payload headers: VP8 S bit + partition 0 + P bit; H.264 IDR or SPS as a single NAL, inside a STAP-A, or as the first FU-A fragment. **Delta frames get no boundary or count.** No payload is reassembled anywhere in the repo.
- **Who already uses it:** first-keyframe join time (`rtpstats.go:206`, `viewer.go` readLoop) and keyframe intervals.

### Clocks

- Publisher: `time.Now()` per packet, marshalled into abs-capture-time NTP (`publisher.go:254`). The conversion drops the monotonic reading.
- Viewer: `now := time.Now()` right after `ReadRTP` (`viewer.go:312`). Arrival is `now.Sub(origin)`, which is monotonic. Transit is `now.Sub(ac.CaptureTime())`, which is wall clock against wall clock.
- No time is passed between publisher and viewer other than inside the packet. There is no shared map or channel (repo-wide grep).

---

## Constraints

| Constraint | Source | Impact |
|---|---|---|
| H.264 frames in memory are Annex-B with 4-byte start codes, and still contain SPS, PPS and SEI | `internal/clip/clip.go:178-193` | `Frame.Data` is not the byte form Q8 hashes |
| pion's `H264Payloader` drops AUD (9) and filler (12), holds SPS/PPS and sends them as a STAP-A with the next NAL (dropped silently if over the MTU), and sends SEI as an ordinary NAL | `rtp@v1.10.5/codecs/h264_packet.go:106-141` | What reaches the wire is not `Frame.Data` byte for byte for H.264 |
| pion's `H264Packet` depacketiser emits Annex-B start codes by default (length prefixes with `IsAVC`), unpacks STAP-A, and reassembles FU-A only on the E bit | `rtp@v1.10.5/codecs/h264_packet.go:214-310` | Its output carries start codes and non-VCL NALs |
| `H264Packet` never checks the FU-A S bit and does not reset its buffer on S, so a lost end fragment leaves stale bytes for the next FU. STAP-B, MTAP and FU-B return `errUnhandledNALUType` | `rtp@v1.10.5/codecs/h264_packet.go:297`, `:313` | Behaviour of the stock depacketiser under loss |
| `H264Packet.IsPartitionHead` is the S bit for FU packets and **true for every other packet** | `rtp@v1.10.5/codecs/h264_packet.go:329-339` | A lost leading single-NAL packet is not detected as a missing head |
| VP8 `IsPartitionHead` checks the S bit only, not PID=0 (looser than RFC 7741 §4.5.1). `IsPartitionTail` is the marker bit for both codecs | `rtp@v1.10.5/codecs/vp8_packet.go:234-239`, `codecs/common.go:30-32` | Frame-end detection in pion leans on the marker |
| pion `samplebuilder` ends a frame on the marker or a timestamp change, never assembles across a sequence gap, and **pops a finished frame only once the following packet has arrived** | `webrtc/v4@v4.2.22/pkg/media/samplebuilder/samplebuilder.go:229-254` | Pop time is about one packet (normally a frame interval) after the frame's last packet |
| `samplebuilder` never sets `media.Sample.Timestamp` and exposes no per-packet arrival time. Its hooks are `WithPacketReleaseHandler` and `WithPacketHeadHandler`. `maxLate` is counted in sequence numbers, and `WithMaxTimeDelay` adds a time bound | `samplebuilder.go:62`, `:155-179`, `:311-318`, `:371-394` | Arrival times must be recorded by the caller at `ReadRTP` |
| RTX is deliberately not registered. NACK is negotiated, and pion's default interceptors run on both clients | `internal/rtc/rtc.go:31-38`, `:108-109` | A retransmission arrives with its original seq on the original SSRC |
| The viewer discards the `ReadRTP` attributes. A redundant copy counts as a duplicate (`rtpstats.go:72-74`). There is no reorder counter | `internal/viewer/viewer.go:308` | The viewer has no marker that tells a retransmitted packet from an original |
| `rtpstats.Stream` is not safe for concurrent use. Access is serialised by `progress.mu` in `readLoop` | `internal/rtpstats/rtpstats.go:22-23`, `internal/viewer/viewer.go:303-340` | Per-packet viewer work happens under one mutex on the read goroutine |
| The publisher's sequence number and timestamp base are random per run (`rand.UintN`, `rand.Uint32`) | `internal/publisher/publisher.go:211-212` | Loop index is not readable from the absolute RTP timestamp without knowing the base |
| The schedule slips if more than 500 ms behind: `ScheduleSlips++` and the origin resets, with no catch-up | `internal/publisher/publisher.go:229-233` | Wall send time and the frame index k can drift apart |
| `publish` mode writes no report and has no metrics. `view` mode has no publisher in-process. Only `run` has both in one process | `cmd/whipbench/main.go:168-218`, `:221`, `internal/runner/runner.go:46-149` | Only `run` shares one process and one clock |
| `report_test.go:118-119` fails if the JSON contains `one-way delay`. `metrics_test.go:41-42` fails if the exposition contains `one_way_delay`. `report_test.go:115` fails on `latency` | `internal/report/report_test.go:104-119`, `internal/metrics/metrics_test.go:41-42` | Existing tests guard these names today |
| Dependencies need a written reason; today only pion/webrtc v4. No cgo | `CLAUDE.md:58-59`, `:123-125` | Hashing is limited to the standard library or a justified dependency |
| Verification: `gofmt -l .`, `go vet`, `go test -race -count=1 ./...`, golangci-lint v2.12.2 (staticcheck and unused disabled on Go 1.27), `scripts/check-repo.sh`, leakcheck, `npm run backlog && npm run build:site` | `CLAUDE.md:76-79`, `:90-99` | The gate for the Implement phase |
| No metric may be called "latency". One-way delay is network plus server, not glass-to-glass | `CLAUDE.md:55-57` | Naming rule |
| A PR carries a `WB-n` subject and a CHANGELOG line under [Unreleased], with no attribution trailers | `CONTRIBUTING.md:51-53` | Landing rule |

---

## Reuse candidates

What already exists and should not be rewritten:

| What | Path | Note |
|---|---|---|
| NAL splitting without start codes | `internal/clip/clip.go:200` `SplitAnnexB` | Already returns NAL units without prefixes. Tested by `TestSplitAnnexB` |
| Access-unit grouping (§7.4.1.2.3) | `internal/clip/clip.go:159-197` | Defines what one H.264 frame is in the clip |
| Loop length and per-frame duration | `internal/clip/clip.go:43-50` | `Duration()` and `FrameDuration()` |
| Frame index → timestamp, continuous across loops | `internal/clip/clip.go:53-65` | Tested across the loop point and the uint32 wrap |
| Keyframe start from payload headers | `internal/rtpstats/rtpstats.go:264` `KeyframeStart` | Codec-aware, tested on real payloads |
| Marker + timestamp + seq-count frame completeness | `internal/rtpstats/rtpstats.go:233` `keyframes.add` | Keyframes only today |
| pion depacketisers and `samplebuilder` | `rtp@v1.10.5/codecs`, `webrtc/v4@v4.2.22/pkg/media/samplebuilder` | Already in the module graph, unused by the repo. The eval probe used them (eval `:11`) |
| Histogram and merge | `internal/stats/stats.go:62-150` | Fixed memory per viewer, exact merge, 0.5% quantiles |
| Unavailable-with-reason shape and aggregation | `internal/viewer/viewer.go:54-68`, `:347`; `internal/report/report.go:110-115`, `:220-232`, `:256` | Includes the "pooled over X of Y" reason |
| Markdown row and "unavailable" line | `internal/report/markdown.go:46-72` | n/p50/p95/p99/min/max table |
| Shared in-process objects in `run` | `internal/runner/runner.go:66`, `:82`, `:132` | `*metrics.Live` is already passed to both publisher and viewers |
| Test relay with loss injection | `internal/testserver/testserver.go:31-41`, `:196-225` | `DropEvery`, `StripExtensions`, `MaxViewers`. Copies marker, seq, timestamp and payload; keeps only abs-capture-time |

---

## Existing tests

| What it covers | Path | How to run it |
|---|---|---|
| Embedded clips: 120 frames, Ticks 3000, keyframe every 30, loop closes on a keyframe; timestamps across loops; `SplitAnnexB`; access-unit grouping | `internal/clip/clip_test.go:10-76` | `go test ./internal/clip` |
| Loss, sequence wrap, reordering, duplicates, jitter (RFC 3550), stalls, keyframe interval and first complete keyframe, `KeyframeStart` on real pion-packetised payloads | `internal/rtpstats/rtpstats_test.go:37-233` | `go test ./internal/rtpstats` |
| Nearest rank, histogram 0.5% bound, exact merge, extremes | `internal/stats/stats_test.go:9-71` | `go test ./internal/stats` |
| Transit unavailable is never a number; key names (`packetTransit` ×4, no `latency`, no `one-way delay`); verdict boundaries; host-only endpoints; ramp offsets present or absent | `internal/report/report_test.go:43-209` | `go test ./internal/report` |
| Exposition series names; no `one_way_delay`; nil `Live` is a no-op | `internal/metrics/metrics_test.go:9-49` | `go test ./internal/metrics` |
| End to end through `testserver`: VP8 and H.264 round trips with transit available, stripped extension → unavailable, lossy relay (~2% at `DropEvery` 50), no verdict, `/metrics` during a run | `internal/runner/runner_test.go:84-207` | `go test ./internal/runner` |
| CLI usage, bearer env, `run` writes both reports with no secrets | `cmd/whipbench/main_test.go:17-111` | `go test ./cmd/whipbench` |
| None | `internal/publisher`, `internal/viewer`, `internal/rtc`, `internal/testserver` | No test files of their own; covered only through the runner tests |

There are no golden-file tests: every report-shape check uses `strings.Contains` or `strings.Count`.

---

## Blind spots

Things that could not be determined, and why:

- **Whether the server-side NACK responder resends a packet the test relay dropped.** `testserver.forward` skips `WriteRTP` for the dropped packet (`testserver.go:210-211`). Whether pion's responder can still resend it depends on pion internals that were not read. So it is unknown whether `DropEvery` produces retransmissions, unrecoverable holes, or both.
- **SSRC and payload type on the relay legs.** `testserver.forward` does not copy them, and what `TrackLocalStaticRTP` sets was not verified.
- **The `maxLate` and options the eval probe used** for `samplebuilder`. `evals/2026-10-02-mediamtx-fingerprint.md` does not give them.
- **MediaMTX re-packetisation, SPS/PPS repetition, SEI or AUD insertion.** The eval compared reassembled VCL bytes and markers per timestamp only (`:28-34`), with one server, one viewer, 15 s and no loss (`:38-39`). Every other server is untested (WB-4).
- **ITU-T H.264 Table 7-1** (VCL types 1-5, non-VCL 6-12) was not reachable (404). The type list comes from pion's comments (`webrtc/v4@v4.2.22/pkg/media/h264reader/nalunittype.go:13-30`) and is UNVERIFIED against the standard. The clip uses only types 1, 5, 6, 7, 8 (no data partitions 2-4).
- **RFC 7741 §4.1**: that every packet of a frame shares the timestamp comes from a fetch summary, not a verbatim quote. RFC 6184 §5.1 (marker on the last packet of the access unit) and RFC 7741 §4.1 (marker on the last packet of each frame), §4.2 (S bit) and §4.5.1 (frame complete) were read.
- **Whether a clip passed with `--clip` can contain byte-identical frames.** Both embedded clips have 120 distinct frames (SHA-1 of each payload, and the eval's SHA-256 over VCL bytes, eval `:16-18`). Arbitrary user clips were not examined.
- **The CPU cost** of reassembly and hashing per viewer. Not measured (out of scope, WB-25).
- **Line coverage** of `internal/viewer` and `internal/publisher`. No coverage run was done.

---

## Facts that contradict the assumptions

- **Q4 — THE VIEWER CANNOT TELL A RETRANSMITTED PACKET FROM AN ORIGINAL TODAY.** RTX is deliberately not registered (`internal/rtc/rtc.go:31-34`), so a NACK retransmission arrives with its original sequence number on the original SSRC. `readLoop` discards the `ReadRTP` interceptor attributes (`internal/viewer/viewer.go:308`), and `rtpstats` has no reorder counter. "Frames that needed a retransmission are counted" therefore has no existing signal to count from. A late packet inside the window looks the same whether it was retransmitted or reordered.
- **Q4 — THERE IS NO REASSEMBLER IN THE REPO, AND PION'S DOES NOT GIVE THE TIME Q4 NEEDS.** No samplebuilder, jitter buffer or depacketiser is used anywhere (`internal/`, `cmd/`). pion's `samplebuilder` pops a frame only once the next packet has arrived (`samplebuilder.go:249-254`), does not record arrival times (`:311-318`), and gives up on a frame by sequence count (`maxLate`) or an optional time bound (`WithMaxTimeDelay`), not by a fixed window alone (`:155-179`, `:389-394`). Separately, `README.md:73` assigns "its sample window and retransmissions" to **WB-41**, while Q4's default puts a fixed window and a retransmission count in this task.
- **Q6 — THE PREMISE IS ALREADY HALF TRUE.** No key `oneWayDelay` or `source` exists. The packet-transit figure already lives under its own key, `packetTransit`, at aggregate (`internal/report/report.go:110-115`) and per viewer (`internal/viewer/viewer.go:54-68`), after a rename from `latency` (`CHANGELOG.md:24-30`). Two existing tests fail the moment the new name appears: `internal/report/report_test.go:118-119` (`one-way delay` in the JSON, message: reserved for WB-38) and `internal/metrics/metrics_test.go:41-42` (`one_way_delay` in the exposition).
- **Q8 — THE H.264 FRAME THE PUBLISHER HOLDS IS NOT THE HASHED FORM, AND THE WIRE IS NOT THE CLIP BYTE FOR BYTE.** `Frame.Data` is an Annex-B access unit with 4-byte start codes, SPS, PPS and SEI (`internal/clip/clip.go:178-193`). The real clip has SPS+PPS before every IDR and one 699-byte SEI in frame 0 only (`scripts/make-clips.sh:29-33`, `repeat-headers=1`). pion's payloader drops AUD and moves SPS/PPS into a STAP-A (`rtp@v1.10.5/codecs/h264_packet.go:106-141`). `README.md:58` says frames "go on the wire byte for byte", which holds for VP8 and for H.264 VCL units only. The eval matched H.264 only on VCL bytes, 450 of 450 (`evals/2026-10-02-mediamtx-fingerprint.md:17-26`). This is consistent with the Q8 default; it contradicts any reading of "the frame" as `Frame.Data`.
- **Q1 — THE LOOP LENGTH IS ONLY PARTLY READ FROM THE CLIP.** For VP8 it comes from the IVF header (`clip.go:140-151`). For H.264 the frame rate is **hard-coded to 30** in `clip.Load` (`internal/clip/clip.go:231`), because Annex-B carries no timing. `Clip.Duration()` therefore gives 4 s for any 120-frame H.264 clip whatever its real frame rate. Relevant facts for "evidence of its own": the RTP timestamp keeps increasing across loops (`clip.go:63-65`), but its base is random per run (`publisher.go:211-212`). The publisher resets its schedule on a slip of more than 500 ms (`publisher.go:229-233`).
- **Q2 — A SPLIT RUN ALREADY EXISTS, BUT NOT AS A SINGLE REPORT.** `publish` (`cmd/whipbench/main.go:168`) writes no report, and `view` (`:221`) writes a report with no publisher section. A split run is therefore a `view` report, and its only delay figure today is packet transit, which `report.go:294` allows "on one host or synchronised clocks". This is consistent with the Q2 default; the unavailable reason would be emitted from a `view` report.
- **Q9 — MediaMTX KEEPS THE MARKER, AND PION ASSUMES ONE.** Every RTP timestamp carried exactly one M=1 packet on both codecs (`evals/2026-10-02-mediamtx-fingerprint.md:31-34`). The publisher sets the marker itself (`internal/publisher/publisher.go:249`). The no-marker path is not exercised by any server measured so far, and `internal/testserver` copies the marker unchanged (`testserver.go:196-225`), so no existing test produces a markerless stream.
- **Q7 — THE EXCLUSION SET IS EMPTY FOR BOTH EMBEDDED CLIPS.** All 120 frames of each clip are byte-distinct (VP8 payloads, and H.264 both as `ParseH264` builds them and as VCL-only bytes). `scripts/make-clips.sh:7` uses `testsrc2`, which draws a frame counter. A clip passed with `--clip` (`cmd/whipbench/main.go:150-159`) is not checked for duplicates today.

---

## Status

- [x] Research complete
- [x] Self-contained (explicit paths, no reference to session context)
- [x] Zero solution proposals
- [x] Reviewed (2026-10-03, Allan Nava, in chat: "ok procedi con il design" — the four contradictions read in summary)

> **Compression ratio:** <tokens burned> → <artifact tokens> = <N>×
> Next phase: **Design**. It receives: this file + `00-questions.md` + the ticket.
