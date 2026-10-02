# 01 · Research — WB-1 Latency method

**Written against:** `2fc8fad`

---

## Reference questions

From `thoughts/WB-1-latency-method/00-questions.md` — all ten defaults accepted (2026-10-01, Allan Nava):

- Q1 → two separately named metrics, never blended: network one-way delay (ms, p50/p95/p99) ranks servers; glass-to-glass sits beside it, labelled, never ranks.
- Q2 → at least one method survives a server that forwards only the RTP payload; a stripped extension is reported as "network stamp unavailable: extension not negotiated (publisher leg / viewer leg)", never blank or zero.
- Q3 → WB-1 is a decision record plus follow-up `BACKLOG.md` tickets; the only code change is renaming the 0.0.1 metric so it stops claiming to be "latency".
- Q4 → abs-capture-time stays the network stamp, once per frame, UQ32.32 from the publisher wall clock; no custom extension; report says per leg forwarded / rewritten / dropped.
- Q5 → glass-to-glass uses a machine-readable block code with a 64-bit ms timestamp drawn into the pixels, read on at most 10 sampled viewers, unreadable-code count reported.
- Q6 → the in-frame endpoint is decoder output, no buffering beyond frame reassembly; called "capture-to-decode", not glass-to-glass.
- Q7 → one host sharing one wall clock is the reference set-up; multi-host allowed with a ±1 ms budget, above it the run is "not comparable", not rejected.
- Q8 → whipbench measures the inter-host offset itself (NTP-style exchange at run start and end, offset ± RTT/2, linear interpolation) and prints every cross-host figure as value ± uncertainty, naming the method.
- Q9 → both methods reported side by side with their own sample counts, no reconciliation; "methods disagree" flag when in-frame p50 < network p50 by more than the combined clock uncertainty.
- Q10 → one sample per frame at its last packet (marker bit), ms at 0.1 ms resolution; count, p50, p95, p99, max; first 5 s after first media excluded; loss and NACK/RTX counts beside.

Out of scope (from the same file): implementing the methods; physical glass-to-glass; audio and lip-sync; transcoding, simulcast, SVC; PTP/GPS time; RTT/2 as one-way delay; signalling latency and time-to-first-frame.

---

## Map of the territory

### Components involved

| Area | Path | Role |
|---|---|---|
| Publisher | `internal/publisher/publisher.go` | WHIP client: streams a pre-encoded clip, paces frames, stamps abs-capture-time on every packet |
| Clip | `internal/clip/clip.go`, `clips.go`, `testdata/clip-vp8.ivf`, `testdata/clip-h264.h264` | parses the embedded IVF / Annex-B clips; nothing is encoded or decoded (`clip.go:1-8`) |
| RTC set-up | `internal/rtc/rtc.go` | MediaEngine codecs, abs-capture-time registration, pion default interceptors, `ExtensionID` lookup |
| Viewer | `internal/viewer/viewer.go` | WHEP client: `ReadRTP` loop, arrival stamping, delay sample, join timings, `Latency` struct |
| RTP stats | `internal/rtpstats/rtpstats.go` | loss (RFC 3550 A.1), jitter (A.8), stalls, keyframe completeness via marker bit |
| Stats | `internal/stats/stats.go` | `Summary` {n,min,mean,p50,p95,p99,max}, exact `Summarise`, 1 % log `Histogram` |
| Report | `internal/report/report.go`, `internal/report/markdown.go` | JSON schema `whipbench.report/v0`, `Aggregate`, `Method` definition strings, Markdown |
| Live metrics | `internal/metrics/metrics.go` | hand-written Prometheus exposition, incl. `whipbench_one_way_delay_seconds` |
| Runner | `internal/runner/runner.go` | one process: optional publisher + N viewer goroutines, warm-up, timeline |
| Scenario | `internal/scenario/scenario.go` | scenario JSON parse, defaults, validation |
| CLI | `cmd/whipbench/main.go` | subcommands `publish`, `view`, `run`, `version`, `help` |
| WHIP/WHEP HTTP | `internal/whip/whip.go` | SDP offer POST, `Host` / `Scrub` |
| Test relay | `internal/testserver/testserver.go` | in-process pion relay for tests; fault options |
| Evidence | `evals/2026-10-01-mediamtx-local.md` + `evals/2026-10-01-mediamtx-local/*` | the only live run: MediaMTX v1.21.1, one machine |

### Entry points

| Symbol | Path:line | What it does |
|---|---|---|
| `Stream` (stamp) | `internal/publisher/publisher.go:253-257` | per packet: `rtp.NewAbsCaptureTimeExtension(time.Now())`, `SetExtension`, then `TrackLocalStaticRTP.WriteRTP` |
| `Stream` (pacing) | `internal/publisher/publisher.go:213-243` | frame k due at origin + k·FrameDuration; packets of a frame written back to back; lag > 500 ms resets origin, `ScheduleSlips++` (:229-233) |
| `Stream` (marker) | `internal/publisher/publisher.go:248-252` | every packet of a frame shares its RTP ts; marker only on the frame's last packet |
| `Connect` | `internal/publisher/publisher.go:157-160` | after `SetRemoteDescription`, stamping on only if the sender's negotiated params carry the URI and `!NoStamp`; `Result.Stamped` |
| `readLoop` | `internal/viewer/viewer.go:297-340` | arrival `now := time.Now()` (:307); delay `now.Sub(ac.CaptureTime())` per packet (:328) |
| `OnTrack` | `internal/viewer/viewer.go:221-222` | extension ID from `receiver.GetParameters().HeaderExtensions`; sets `Latency.Negotiated` |
| `finishLatency` | `internal/viewer/viewer.go:342-358` | decides `available` / `reason` |
| `NewAPI` | `internal/rtc/rtc.go:84-109` | registers codecs, the abs-capture-time URI (:105), `webrtc.RegisterDefaultInterceptors` (:108-109) |
| `ExtensionID` | `internal/rtc/rtc.go:127-134` | negotiated ID for a URI, 0 when the answer dropped it |
| `Build` / latency merge | `internal/report/report.go:104-114`, `:199-227` | merges per-viewer delay histograms into `aggregate.latency` |
| `Method` lines | `internal/report/report.go:286-291` | definition strings copied into every report (latency at :290) |
| `Live.Delay` | `internal/metrics/metrics.go:19`, `:51-63`, `:93-102` | Prometheus delay histogram, buckets 1…5000 ms |
| `Histogram`, `Quantile`, `Merge` | `internal/stats/stats.go:55-75`, `:127-147`, `:111-122` | pooled per-packet delay statistics |
| `keyframes.add` | `internal/rtpstats/rtpstats.go:233-254` | keyframe complete when key-start seq … marker seq all present |
| `forward` | `internal/testserver/testserver.go:196-225` | rebuilds each viewer header; seq, ts, marker unchanged; only abs-capture-time remapped |
| `execute` | `cmd/whipbench/main.go:251-284` | stdout summary: join p50/p95, loss, "one-way delay" p50/p99 or the unavailable reason |

---

## Existing patterns and conventions

### Unavailable with a reason, never a fake number

- **Where:** `internal/viewer/viewer.go:55-68`, `:342-358`; `internal/report/report.go:109-114`; `internal/report/markdown.go:77`; rule 2 in `CLAUDE.md:45-47`.
- **How it works:** `latency{available:false, reason}` with `ms` omitted (nil pointer). Reasons in order: not negotiated; negotiated but no stamps arrived ("server strips or rewrites"); invalid stamps > `MaxInvalidStampShare` = 1 % (`viewer.go:39`), blamed on unsynchronised clocks or a rewriting server (`viewer.go:349`); no valid samples. A sample < 0 or > `MaxPlausibleDelay` = 60 s (`viewer.go:35`) counts as `invalid`.
- **Who already uses it:** `internal/runner/runner_test.go:130`, `internal/report/report_test.go:88`, `cmd/whipbench/main.go:251-284`.

### A definition lives in three places, and comes before code

- **Where:** `CLAUDE.md:41-44` (rule 1), `CONTRIBUTING.md:19-29`.
- **How it works:** a measurement's definition must agree in the README table (`README.md:45-54`), the `Method` strings (`internal/report/report.go:286-291`) and the package comment of the code computing it.
- **Who already uses it:** one-way delay (`README.md:53`, `README.md:62-71`, `report.go:290`, `metrics.go:93-94`).

### Two kinds of summary

- **Where:** `internal/stats/stats.go:26-53` (nearest-rank, exact) and `:55-147` (log histogram, growth 1.01 from 1 µs to 60 s, quantile = bucket geometric midpoint clamped to [min,max], min/max/mean exact).
- **How it works:** join, signalling, loss, jitter take one value per joined viewer, exact; delay pools every packet of every viewer in the histogram (`report.go:291`, `README.md:56`).
- **Who already uses it:** `internal/report/report.go:92-114`, `internal/viewer/viewer.go:65`.

### Negotiation is read back after `SetRemoteDescription`

- **Where:** `internal/rtc/rtc.go:127-134` over `GetParameters().HeaderExtensions`.
- **How it works:** sender side `internal/publisher/publisher.go:158`, receiver side `internal/viewer/viewer.go:221`. No SDP parsing or munging anywhere; the offer is POSTed verbatim (`internal/whip/whip.go:42-81`).

### Clocks: monotonic within a process, wall across the stamp

- **Where:** `internal/viewer/viewer.go:246`, `:307-308` (join timings from `origin = time.Now()` at the WHEP POST — monotonic) versus `:328` (delay against `CaptureTime()`, built with `time.Unix`, no monotonic reading — wall clock on both ends).

### Naming and schema

- JSON keys are lowerCamelCase with a unit suffix `Ms` / `S` / `Percent`; aggregate names differ from per-viewer names (`joinFirstRtpMs` vs `firstRtpMs`, `report.go:93`, `viewer.go:88`).
- `Schema = "whipbench.report/v0"` with "bump it when a field changes meaning" (`internal/report/report.go:30-31`).
- Prometheus series carry the `whipbench_` prefix (`internal/metrics/metrics.go:6`). No written naming rule exists beyond these.

### Test on synthetic input, then through the relay

- `CONTRIBUTING.md:23-24`: a measurement is tested on synthetic input in `internal/rtpstats` or `internal/stats` first; end-to-end tests run against `internal/testserver` with fault options (`testserver.go:31-41`: `StripExtensions`, `DropEvery`, `MaxViewers`, `Token`).

### Dated external facts and repo process

- `CLAUDE.md:59-85` keeps "Facts the code depends on (dated)"; MediaMTX's negotiation is one (`CLAUDE.md:71-73`).
- Backlog items are `WB-n` with `<!-- wb: prio= size= labels= [ver=] -->` (`CONTRIBUTING.md:40-45`); labels include `measurement`, `report`, `research` (`package.json:13-58`); `ROADMAP.md` is generated (`npm run roadmap`).
- CHANGELOG is Keep a Changelog 1.1.0, line under `[Unreleased]` in the same PR, items cite `WB-n` (`CHANGELOG.md:3-4`, `CONTRIBUTING.md:49-51`).

---

## Constraints

### In the repository

| Constraint | Source | Impact |
|---|---|---|
| Pure Go, no cgo; a decoder must respect it or argue against it in Design | `CLAUDE.md:41-57` (rule 6); `.github/workflows/ci.yml:19-39` builds `CGO_ENABLED=0` for linux/darwin × amd64/arm64 | rules out libvpx/ffmpeg/openh264 bindings unless Design argues otherwise |
| Every dependency needs a written reason in CLAUDE.md; today only pion | `CLAUDE.md:118-120`, `go.mod:6-30` | `go.mod` has no decoder, image or OCR module |
| Publisher streams pre-encoded clips only, byte for byte; no encoder in the repo | `internal/clip/clip.go:1-8`, `README.md:58`, `README.md:123` | nothing can be drawn into frames at run time; PLI/FIR cannot be answered |
| Clips: `testsrc2` 640×360, 30 fps, 4 s, fixed GOP 30, 600 kbit/s; VP8 libvpx realtime; H.264 baseline 42e01f, one slice, no B-frames | `scripts/make-clips.sh:7-33` | one bitrate exists; the scenario has no bitrate, fps or resolution field (`internal/scenario/scenario.go:28-62`) |
| H.264 clip is parsed at a hard-coded 30 fps | `internal/clip/clip.go:231` | — |
| RTP ts = base + k·Ticks, continuous across loops; random base and initial seq | `internal/clip/clip.go:63-65`, `internal/publisher/publisher.go:211-212` | frame index k and RTP ts map one to one within a run |
| RTX deliberately not negotiated; NACK, PLI, FIR, REMB feedback are | `internal/rtc/rtc.go:31-37` | a retransmission arrives on the original SSRC and seq; loss is measured after NACK recovery (`report.go:287`) |
| No NACK, RTX or PLI counters anywhere in the report; pion's stats interceptor is registered by default but never read | `internal/rtpstats/rtpstats.go:154-174`, `internal/rtc/rtc.go:108-109`; no `GetStats` / stats `Getter` use in the repo | — |
| Nothing is excluded at the start of a run; `warmupSeconds` (default 2) is the gap between the publisher connecting and the first viewer start | `internal/scenario/scenario.go:43-45`, `:100`; `internal/runner/runner.go:94-100`; `internal/viewer/viewer.go:307-336` | the name `warmupSeconds` is taken, with that meaning |
| "Joined" = last packet of the first complete keyframe within `joinTimeoutSeconds` (default 10), timed from the WHEP POST | `internal/viewer/viewer.go:246`, `:319-322`; `README.md:47-49`; `report.go:286` | first media = first RTP packet (`firstRtpMs`) |
| Report schema `whipbench.report/v0`, bumped when a field changes meaning | `internal/report/report.go:30-31` | — |
| Report has no host identity and no per-leg breakdown: `client{os,arch,cpus,go}`, `server{whipHost,whepHost}` | `internal/report/report.go:58-69` | a split-host run is not distinguishable from a single-host run in the JSON |
| One process only; no distributed mode. `publish` and `view` are separate subcommands, so publisher and viewers can already run on different machines, with no clock handling and no channel between them | `internal/runner/runner.go:46-144`; `cmd/whipbench/main.go:71-87`, `:142-208` | multi-machine is on the v0.4.0 line of `README.md:146` |
| Every viewer gets a full per-viewer record; no sampling of viewers | `internal/report/report.go:53`, `:155` | — |
| Literal strings checked by CI | `scripts/check-repo.sh:15-18` (README must contain "not glass-to-glass", "latency: unavailable") | a rename that drops these phrases fails CI |
| Literal strings matched by tests | `internal/report/report_test.go:62`, `:98` and `internal/runner/runner_test.go:143`, `:159` ("**Latency: unavailable**", "one-way delay (stamped"); `internal/metrics/metrics_test.go:9` (exact `whipbench_one_way_delay_seconds_*` lines); `cmd/whipbench/main_test.go:82` (`aggregate` key) | renames break these; Go struct-field renames break at compile time |
| Hosts only, never paths or queries, in a report | `CLAUDE.md:51-53`, `internal/whip/whip.go` `Host` / `Scrub`, `internal/scenario/scenario.go:189-196` | — |
| Histogram accuracy is relative: 1 % buckets, quantiles within 0.5 % (test tolerance 0.6 %) | `internal/stats/stats.go:55-75`, `:127-147`; `internal/stats/stats_test.go:31` | absolute resolution is 0.1 ms only below ~20 ms |

### Outside the repository (verified 2026-10-02)

| Constraint | Source | Impact |
|---|---|---|
| MediaMTX v1.21.1 drops abs-capture-time on both its WHIP and WHEP answers; the WHEP answer keeps `mid`, `rtp-stream-id`, `repaired-rtp-stream-id`, transport-wide-cc | `evals/2026-10-01-mediamtx-local.md:44-48`; per-run `.md` files line 30 and 36; `CLAUDE.md:71-73` | no latency value exists for any real server; GOP passes through at exactly 1.00 s in RTP time (`evals/…md:39`) |
| abs-capture-time: URI `http://www.webrtc.org/experiments/rtp-hdrext/abs-capture-time`; 8-byte UQ32.32 capture timestamp, or 16 bytes with a signed Q32.32 estimated capture clock offset | webrtc.googlesource.com/src/+/refs/heads/main/docs/native-code/rtp-hdrext/abs-capture-time/README.md (no revision date on the page) | the repo sends the 8-byte form |
| Spec semantics: the capture time of the first frame in the packet, on the capture system's NTP clock; "Capture NTP Clock = Sender NTP Clock + Capture Clock Offset" | same | — |
| Spec cadence: a sender SHOULD NOT send it on every packet, SHOULD send it at regular intervals (e.g. every second); receivers extrapolate from RTP timestamps | same | 0.0.1 sends it on every packet |
| Spec intermediaries: a mixer MAY adjust the timestamps or rewrite them completely against its own NTP clock | same | a conforming server may change the value |
| LiveKit SFU by default rewrites abs-capture-time into the SFU clock once a validated SR arrives and strips it before; verbatim forwarding is an opt-in, merged 2026-09-22 | github.com/livekit/livekit/pull/4812, issue #4813 | — |
| SFUs may terminate RTCP and originate their own SRs (RFC 8079 §3.2); mediasoup does so (maintainer, mediasoup discourse thread 3427, 2021-10-14) | rfc-editor.org/rfc/rfc8079 | SR NTP/RTP pairs seen by a viewer are not the publisher's |
| pion/rtp v1.10.5 `AbsCaptureTimeExtension{Timestamp uint64; EstimatedCaptureClockOffset *int64}`, `Marshal`/`Unmarshal`, `CaptureTime()`, `EstimatedCaptureClockOffsetDuration()`, `NewAbsCaptureTimeExtension`, `NewAbsCaptureTimeExtensionWithCaptureClockOffset` | `github.com/pion/rtp@v1.10.5/abscapturetimeextension.go:29`, `:93`, `:98`, `:117`, `:124` | `CaptureTime()` uses `time.Unix`: no monotonic reading |
| No abs-capture-time URI constant in pion webrtc or sdp (sdp has `ABSSendTimeURI` only); the repo defines `rtc.AbsCaptureTimeURI` | `github.com/pion/sdp/v3@v3.0.20/extmap.go:20`; `internal/rtc/rtc.go:18` | — |
| pion/webrtc v4.2.22: negotiated extensions come only from the remote SDP and only for locally registered URIs; after negotiation `GetParameters()` returns only negotiated ones; each PeerConnection copies the MediaEngine | `github.com/pion/webrtc/v4@v4.2.22/mediaengine.go:562-586`, `:738-756`; `peerconnection.go:167` | the doc comment at `mediaengine.go:275` names a `GetHeaderExtensionID` that exists only unexported (`:357`) |
| `TrackLocalStaticRTP.writeRTP` overwrites SSRC and PT and passes caller-set extensions through; `TrackLocalStaticSample` has no extension hook | `track_local_static.go:195-216`, `:343` | — |
| pion/interceptor v0.1.49: packages cc, ccfb, flexfec, gcc, intervalpli, jitterbuffer, nack, pacing, packetdump, report, rfc8888, rtpfb, stats, twcc; no rtx, no abs-capture-time; NTP helpers are under `internal/ntp` and not importable | `github.com/pion/interceptor@v0.1.49/pkg/`, `internal/ntp/ntp.go:13-42` | — |
| SR generation: `NTPTime = ToNTP(now)`, `RTPTime` = last ts + elapsed × clock rate; the receiver interceptor keeps only LSR/DLSR, not the NTP/RTP pair | `interceptor@v0.1.49/pkg/report/sender_stream.go:60-73`, `receiver_stream.go:113-118`; `github.com/pion/rtcp@v1.2.18/sender_report.go:13-26` | pion/rtcp has no NTP↔time helper |
| Stats interceptor exposes per SSRC: PacketsReceived/Lost, Jitter, NACK/PLI/FIR counts, RTT (remote-inbound) | `interceptor@v0.1.49/pkg/stats/received_stats.go:12-62`, `sent_stats.go:26-57`, `interceptor.go:135` | — |
| Go 1.27.1: `time.Now` carries wall + monotonic; `Sub` uses monotonic only when both operands have it; `time.Unix` has none | `$GOROOT/src/time/time.go:10-70`, `:1227-1232` | `now.Sub(CaptureTime())` is a wall-clock difference, exposed to clock steps |
| Pure-Go VP8 decoding: `golang.org/x/image/vp8` (v0.46.0, BSD-3) decodes keyframes only — inter frames fail "Golden / AltRef frames are not implemented" | `golang.org/x/image@v0.46.0/vp8/decode.go:343-346` | — |
| Other pure-Go decoders, all pre-v1 or self-reported: `github.com/gen2brain/vpx/vp8` (BSD-3, v0.2.1, key + inter, amd64 asm); `github.com/liqmix/govid` (MIT, untagged, VP8 + H.264 claims); `github.com/thesyncim/goh264` (LGPL-2.1, untagged, "not production-ready"); `github.com/Eyevinn/hi264` (MIT, IDR + P_Skip only) | pkg.go.dev / GitHub pages, read 2026-10-02 | none is vetted |

---

## Reuse candidates

What already exists and should not be rewritten:

| What | Path | Note |
|---|---|---|
| Negotiated-ID lookup | `internal/rtc/rtc.go:127-134` `ExtensionID` | works on sender and receiver params |
| abs-capture-time URI + registration | `internal/rtc/rtc.go:18`, `:105` | registered for video, both directions |
| Extension type with offset field | `github.com/pion/rtp@v1.10.5/abscapturetimeextension.go:124` | the offset constructor is unused today |
| Availability + reason pattern | `internal/viewer/viewer.go:55-68`, `:342-358`; `internal/report/report.go:109-114` | distinguishes "not negotiated" from "no stamps arrived" |
| Marker-bit frame completeness | `internal/rtpstats/rtpstats.go:233-254` | keyframes only today |
| Keyframe-start payload parsing | `internal/rtpstats/rtpstats.go:264-333` `KeyframeStart` | VP8 descriptor; H.264 single NAL / STAP-A / FU-A |
| Frame reassembly from RTP | `github.com/pion/webrtc/v4@v4.2.22/pkg/media/samplebuilder/samplebuilder.go:62`, `:398` `WithRTPHeaders` | keeps every packet's header (and extensions) on the `Sample`; depacketizers `codecs.VP8Packet`, `codecs.H264Packet` in pion/rtp |
| Exact and histogram summaries, merge | `internal/stats/stats.go:38-53`, `:55-147` | — |
| NACK/PLI/RTT counters | pion stats interceptor, already registered via `rtc.go:108-109` | unread today |
| Fault-injecting test relay | `internal/testserver/testserver.go:31-41` | `StripExtensions`, `DropEvery`; remaps only abs-capture-time |
| Publisher `Result.Stamped`, viewer `Latency.Negotiated` / `Stamped` / `Invalid` | `internal/publisher/publisher.go:48-63`; `internal/viewer/viewer.go:55-68` | per-leg negotiation is already recorded, as booleans and counts |
| `--no-stamp` flag | `cmd/whipbench/main.go:150` | disables the stamp on the publisher |

---

## Existing tests

| What it covers | Path | How to run it |
|---|---|---|
| Round trip VP8 / H.264 through the relay: `Stamped`, `Latency.Available`, p50 > 0, p99 ≤ 1000 ms | `internal/runner/runner_test.go:84`, `:117` | `go test ./internal/runner` |
| Stripped extension → "no stamps arrived", "**Latency: unavailable**" | `internal/runner/runner_test.go:130` | same |
| Loss through a lossy relay, NACK recovery | `internal/runner/runner_test.go:164` | same |
| Metrics endpoint during a run | `internal/runner/runner_test.go:177` | same |
| Unavailable latency is never a number; verdict rules; hosts only | `internal/report/report_test.go:42-120` | `go test ./internal/report` |
| Nearest rank, histogram within 0.5 %, exact merge, extremes | `internal/stats/stats_test.go:9-71` | `go test ./internal/stats` |
| Loss, wrap, reorder, duplicates, RFC 3550 jitter, stalls, keyframe interval and completeness | `internal/rtpstats/rtpstats_test.go:37-233` | `go test ./internal/rtpstats` |
| Exposition strings incl. delay buckets, negative sample ignored | `internal/metrics/metrics_test.go:9`, `:46` | `go test ./internal/metrics` |
| Clips: embedded, ts continuous across loops, Annex-B split | `internal/clip/clip_test.go:10-76` | `go test ./internal/clip` |
| Scenario defaults, field names, validation | `internal/scenario/scenario_test.go:9-69` | `go test ./internal/scenario` |
| CLI usage, report files written, `aggregate.valid` | `cmd/whipbench/main_test.go:17-82` | `go test ./cmd/whipbench` |
| None | `internal/publisher`, `internal/viewer`, `internal/rtc` | no `_test.go` files |
| Whole suite as CI runs it | `.github/workflows/ci.yml:19-39` | `go test -race -count=1 ./...`; plus `scripts/check-repo.sh`, `npm run backlog`, `node scripts/leakcheck.mjs` |

---

## Blind spots

Things you could not determine, and why:

- **No latency value has ever been measured against a real server.** The one live run (MediaMTX v1.21.1) negotiated no stamp on either leg.
- **Which extensions MediaMTX's WHIP answer kept** besides dropping abs-capture-time. The evals record only the WHEP answer's list.
- **Whether MediaMTX preserves the marker bit.** The RTP timestamp evidently survives (the keyframe interval reads exactly 1.00 s in RTP time), but nothing records the marker bit.
- **How other servers treat abs-capture-time and SRs.** Only LiveKit (rewrites by default) and mediasoup (originates its own RTCP) were verified. Janus came from a search snippet. ion-sfu, Galene, Janus streaming, Red5 and others were not checked.
- **Pure-Go VP8 or H.264 encoders were not researched.** One project (`oops1/go.264`) claims an encoder but was not opened. This matters for drawing a code into frames at run time.
- **Pure-Go decoder maturity is self-reported** for every candidate except `x/image/vp8`. It was also not checked whether `gen2brain/vpx` builds with `CGO_ENABLED=0` on arm64.
- **CPU cost of decoding per viewer** has not been measured anywhere.
- **What `rtpstats` counts as `tooLate`**, and whether a NACK-recovered packet lands there, was not traced (`internal/rtpstats/rtpstats.go:155-162`).
- **Three pion behaviours were not traced:**
  - whether pion picks the one-byte or the two-byte extension header;
  - extension-ID remapping on renegotiation;
  - whether the stats interceptor's RTT is populated without RTCP XR.
- **The abs-capture-time spec page shows no revision date.** The text was read on 2026-10-02.
- **`backlogsync check` is an external tool.** Its rules beyond item format and roadmap freshness are not visible in this repo.

---

## Facts that contradict the assumptions

1. **Q3 and Q1: the 0.0.1 metric already carries the name Q1 gives the new headline.**
   - It is called "one-way delay" in four places:
     - `README.md:53`
     - the Markdown row "one-way delay (stamped packets)" (`internal/report/markdown.go:58`)
     - the Prometheus series `whipbench_one_way_delay_seconds` (`internal/metrics/metrics.go:19`, `:93-94`)
     - stdout (`cmd/whipbench/main.go:272`)
   - "Latency" survives only in a few places: the JSON keys `latency` (`internal/viewer/viewer.go:92`, `internal/report/report.go:104`), the Markdown "Latency: unavailable" heading and "latency p50 / p99" columns, and the CI-checked README phrase "latency: unavailable" (`scripts/check-repo.sh:16`).
   - What it measures is per-packet send stamp to arrival. That is not the per-frame capture-stamped network delay that Q1, Q4 and Q10 define.
2. **Q10: three parts of the default do not match what exists.**
   - **Exclusion:** nothing is excluded at the start of a run today. The name that suggests it, `warmupSeconds`, already means the pause before the first viewer starts (`internal/scenario/scenario.go:43-45`).
   - **RTX counts:** RTX is deliberately not negotiated (`internal/rtc/rtc.go:31-34`), so there is no RTX stream to count. NACK counts exist only inside pion's stats interceptor, which nothing reads.
   - **"0.1 ms resolution":** the only pooled-delay statistic is a relative 1 % histogram (quantiles within 0.5 %, `internal/stats/stats.go:55-147`). That is coarser than 0.1 ms above about 20 ms, for example ±1 ms at 200 ms.
3. **Q5 and Q6: the frame-code method has nothing to stand on in the repo or its allowed dependencies.**
   - The publisher cannot draw anything: it sends pre-encoded clips byte for byte and the repo has no encoder (`internal/clip/clip.go:1-8`).
   - The pure-Go rule (`CLAUDE.md` rule 6) excludes the usual decoders.
   - The only first-party pure-Go VP8 decoder decodes keyframes only (`golang.org/x/image@v0.46.0/vp8/decode.go:343-346`). So "read every frame" on VP8 cannot use it, and every alternative is pre-v1 or self-reported.
   - Q5's "lowest bitrate any scenario uses" has no counterpart: scenarios carry no bitrate, and the only bitrate is the clips' fixed 600 kbit/s (`scripts/make-clips.sh:24-33`).
4. **Q4: two tensions with the spec. Neither contradicts the default, but both bear on it.**
   - **Cadence:** the spec says a sender SHOULD NOT stamp every packet and suggests regular intervals such as every second, with receivers extrapolating from RTP timestamps. Per-frame stamping is 30 times that example at 30 fps.
   - **Rewriting:** this is the default behaviour of at least one major SFU, LiveKit, which rewrites into its own clock after an SR (PR #4812), not an edge case. The spec explicitly allows intermediaries to rewrite.
5. **Q7: split-host runs already exist, but nothing marks them.** `publish` and `view` are separate subcommands, so publisher and viewers can run on different machines today. When they do, the report has no host identity or clock field to say so (`internal/report/report.go:58-69`). Such a run is indistinguishable from the single-host reference set-up.

Holding as assumed:

- Q2: MediaMTX strips the extension on both legs (`evals/2026-10-01-mediamtx-local.md:44-48`).
- Q8: there is no clock-offset code, so the repo has none to replace.
- Q9: there is no reconciliation logic.
- Q6: the viewer has no jitter buffer (`internal/viewer/viewer.go:10-11`).

---

## Status

- [x] Research complete
- [ ] Self-contained (explicit paths, no reference to session context)
- [ ] Zero solution proposals
- [ ] Reviewed (<date>, <who>)

> **Compression ratio:** ~0.9M tokens burned (eight subagent runs, four of them repeats after an interrupted hand-back) → ~7k artifact tokens = ~130×
> Next phase: **Design**. It receives: this file + `00-questions.md` + the ticket.
