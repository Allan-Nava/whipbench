# 2026-10-04 — what MediaMTX, OvenMediaEngine, LiveKit and Janus forward (WB-4)

WB-38's fingerprint needs a server to forward each frame's bytes unchanged and to keep the
marker bit on its last packet; WB-39's stamp source needs abs-capture-time negotiated on both
legs and forwarded. This records, per server, what was checked, how, and what was not. The
2026-10-02 MediaMTX check (`2026-10-02-mediamtx-fingerprint.md`) is the precedent; this one
uses whipbench's own fingerprint figure instead of a throwaway viewer.

| | |
|---|---|
| machine | one Apple-silicon laptop, Docker 24.0.6, every server in a local container |
| whipbench | `5286f61` (main after WB-43), `run` with one viewer, 15 s hold, per codec |
| extension lists | a throwaway pion program, not part of whipbench: whipbench's own media engine (`rtc.NewAPI`), one `sendonly` (WHIP) or `recvonly` (WHEP) video transceiver, the offer POSTed with `whip.Offer`, the `a=extmap` lines of offer and answer printed |
| offer extensions | abs-capture-time, `sdes:mid`, `sdes:rtp-stream-id`, `sdes:repaired-rtp-stream-id`, transport-wide-cc (pion's defaults plus whipbench's stamp) |

## Summary

| server | WHIP | WHEP | abs-capture-time | frame bytes | marker bit |
|---|---|---|---|---|---|
| MediaMTX v1.21.1 | yes | yes | negotiated on neither leg | unchanged, both codecs (2026-10-02) | kept, both codecs (2026-10-02) |
| OvenMediaEngine v0.21.0 | yes | **no** — HTTP 404 | not negotiated on WHIP | not observable: no WHEP | not observable |
| LiveKit | only through Ingress, which transcodes | **no** | not checked | not checked | not checked |
| Janus 1.1.2 + Meetecho WHIP/WHEP 1.1.0 | yes | yes, through a Streaming mountpoint | negotiated on neither leg | unchanged, both codecs | kept, both codecs |

Two consequences for the plan. **No server tested forwards abs-capture-time**, so WB-39's stamp
source would be unavailable on every one of them; the fingerprint is the only one-way delay
these servers allow. And **two of WB-20's four servers cannot be viewed by whipbench at all**:
its viewer speaks WHEP, and OvenMediaEngine and LiveKit do not.

## MediaMTX v1.21.1

`bluenviron/mediamtx:1.21.1`, `MTX_WEBRTCADDITIONALHOSTS=127.0.0.1`, ports 8889 and 8189/udp.
The WHIP answer and the WHEP answer accept the same four extensions, on both codecs:
`sdes:mid`, `sdes:rtp-stream-id`, `sdes:repaired-rtp-stream-id`, transport-wide-cc. Neither
accepts abs-capture-time, as the 2026-10-01 run found. Frame bytes and markers: unchanged and
kept on both codecs, per the 2026-10-02 check.

## OvenMediaEngine v0.21.0

`ovenmedialabs/ovenmediaengine:latest` (built 2026-08-13, banner v0.21.0), default
configuration, `OME_HOST_IP=127.0.0.1`, ports 3333, 3478 and 10000-10004/udp.

- **WHIP** at `/app/<stream>?direction=whip` takes both codecs: `whipbench publish` connected and
  sent for 8 s (VP8) and 20 s (H.264), "send-time stamp negotiated: false" each time. The answer
  accepts `sdes:mid`, `sdes:rtp-stream-id` and transport-wide-cc; not
  `repaired-rtp-stream-id`, not abs-capture-time.
- **WHEP**: none. A WHEP POST to `/app/<stream>?direction=whep` with the stream live answers
  HTTP 404, and `whipbench view` records `http_404`. OvenMediaEngine plays WebRTC through its own
  WebSocket signalling (`ws://host:3333/app/stream`); WHEP is on its 2025 roadmap
  (OvenMediaLabs/OvenMediaEngine discussion #1758) and in no release up to v0.21.0.
- Frame bytes and markers on the way out cannot be checked with whipbench's viewer.

## LiveKit — not run

Read, not run, because neither leg can be measured as it is: LiveKit takes WHIP only through
its separate Ingress service, which transcodes the source by default (docs.livekit.io, "Ingress
overview"), and it has no WHEP egress — livekit/livekit#2811 was closed as not planned, and
#4638 ("WHEP(-ish) Egress Implementation", July 2026) is open. A viewer has to join a room
through LiveKit's own signalling. Whether Ingress forwards WHIP without transcoding when told
to was not checked.

## Janus 1.1.2, with Meetecho's WHIP and WHEP servers

Janus has no WHIP or WHEP of its own; Meetecho's `janus-whip-server` and `janus-whep-server`
(npm, 1.1.0) translate. The image is built from Debian bookworm's `janus` package (1.1.2) and
those two packages — no official image exists. The recipe, reproducible from this directory:
[`2026-10-04-server-forwarding/janus/`](2026-10-04-server-forwarding/janus/) (`Dockerfile`,
`server.js`, both plugin configurations, both scenarios). whipbench ran in a second container
on the same Docker network, so ICE needed no address mapping.

- **The WHEP path.** A WHEP endpoint on the VideoRoom subscribes with a *server* offer — the
  server answers the POST with HTTP 406 and its own offer, the flow of the early WHEP drafts.
  whipbench only ever sends its own offer, so it cannot subscribe that way. The path measured is the one Meetecho's libraries offer for client
  offers: WHIP publishes into VideoRoom 1234, the publisher is RTP-forwarded to a Streaming
  mountpoint (one per codec, ports 5004 and 5006), and WHEP subscribes to the mountpoint. The
  figures are of that chain, not of the VideoRoom alone.
- **Extensions.** The WHIP answer accepts `sdes:mid`, `sdes:rtp-stream-id`,
  `sdes:repaired-rtp-stream-id` and transport-wide-cc; the WHEP answer `sdes:mid` and
  abs-send-time — Janus's own, not in the offer. abs-capture-time on neither leg.
- **Frames and markers**, from whipbench's report (WB-38's fingerprint, every complete frame
  looked up in the clip):

| codec | complete frames | matched (samples) | unmatched | incomplete | frame end | one-way delay p50 / p99 |
|---|---|---|---|---|---|---|
| VP8 | 449 | 449 | 0 | 0 | marker | 0.4 / 1.9 ms |
| H.264 | 448 | 448 | 0 | 0 | marker | 0.8 / 3.0 ms |

Every frame arrived byte for byte (H.264 compared on its VCL NAL units, as WB-38 does), and
the viewer found every frame's end by its marker. Loss 0. The delays are of a local container
on one laptop and say nothing about Janus under load; they are here to show the figure exists.
Reports: [`janus/`](2026-10-04-server-forwarding/janus/).

## Not covered

One machine, one viewer, 15 s per codec; no loss, no simulcast, no transcoding settings. The
OvenMediaEngine and LiveKit legs that whipbench cannot reach were not measured by other means.
The probe's source is not kept; what it printed is quoted above.
