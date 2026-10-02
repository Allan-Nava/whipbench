# 2026-10-02 — frame bytes and marker bits through MediaMTX

Three facts the WB-1 Design (`thoughts/WB-1-latency-method/02-design.md`, D2 and D4) assumed
and the Research had not verified. Checked by hand for the Design review; the probe was a
throwaway program, not part of whipbench.

| | |
|---|---|
| server | MediaMTX v1.21.1 (`bluenviron/mediamtx:1.21.1`), Docker, `127.0.0.1:8889` |
| publisher | `whipbench publish --codec vp8|h264 --include-loopback` (whipbench at `c0370f7`) |
| viewer | a pion v4.2.22 WHEP client: `samplebuilder` with the VP8 / H.264 depacketiser, SHA-256 of every reassembled frame looked up in the set of the clip's frame hashes |
| duration | 15 s per codec, one viewer, one run each |

## The clips' frames are pairwise byte-distinct

Read straight from `testdata/`: `clip-vp8.ivf` has 120 frames, 120 distinct SHA-256 hashes
(keyframes at 0, 30, 60, 90); `clip-h264.h264` has 120 access units (VCL NAL units 1 and 5
joined without start codes), 120 distinct. The source is ffmpeg's `testsrc2`, which already
draws a frame counter into the picture (`scripts/make-clips.sh:7`).

## MediaMTX forwards the frame bytes unchanged

| codec | frames reassembled | matching a sent frame | not matching |
|---|---|---|---|
| VP8 | 451 | 451 | 0 |
| H.264 (VCL NAL units only) | 450 | 450 | 0 |

H.264 was compared on its VCL NAL units only, because a server may add or repeat SPS/PPS;
whether MediaMTX does was not checked.

## MediaMTX keeps one marker bit per frame

Every RTP timestamp carried exactly one packet with the marker bit set, on both codecs,
except the last timestamp of each run — the frame still in flight when the probe stopped.

## Not covered

One server, one machine, one viewer, 15 s; the other WB-4 servers are untested, and so is
behaviour under loss, transcoding or simulcast.
