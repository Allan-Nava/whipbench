#!/bin/sh
# Regenerate the synthetic clips in testdata/ that `whipbench publish` streams.
#
# The clips are committed, so nobody needs ffmpeg to build, test or run whipbench;
# this script is the record of exactly how they were made, and the way to remake them.
#
#   source     ffmpeg's lavfi testsrc2 (a moving pattern with a frame counter), no audio
#   size       640x360, 30 fps, 4 s = 120 frames
#   GOP        a keyframe every 30 frames (1 s), fixed — no scene-cut keyframes, so the
#              interval a viewer observes is exactly the one encoded, and the clip loops
#              on a keyframe boundary (120 is a multiple of 30)
#   VP8        libvpx, ~600 kbit/s, realtime deadline, error-resilient, in IVF
#   H.264      libx264 constrained baseline 3.1 (profile-level-id 42e01f, what every
#              WebRTC stack accepts), one slice per frame, no B-frames, SPS/PPS repeated
#              before every IDR, as an Annex-B elementary stream
#
# Usage: scripts/make-clips.sh   (from anywhere; writes into testdata/)
set -eu
cd "$(dirname "$0")/.."
command -v ffmpeg >/dev/null || { echo "ffmpeg not found" >&2; exit 1; }

SRC="testsrc2=size=640x360:rate=30:duration=4"

ffmpeg -hide_banner -loglevel error -y -f lavfi -i "$SRC" \
  -c:v libvpx -b:v 600k -minrate 600k -maxrate 600k -deadline realtime -cpu-used 8 \
  -error-resilient 1 -g 30 -keyint_min 30 -auto-alt-ref 0 -lag-in-frames 0 \
  -pix_fmt yuv420p -f ivf testdata/clip-vp8.ivf

ffmpeg -hide_banner -loglevel error -y -f lavfi -i "$SRC" \
  -c:v libx264 -profile:v baseline -level 3.1 -preset veryfast -tune zerolatency \
  -b:v 600k -maxrate 600k -bufsize 600k -g 30 -keyint_min 30 -sc_threshold 0 -bf 0 \
  -x264-params "slices=1:repeat-headers=1:sliced-threads=0" -threads 1 \
  -pix_fmt yuv420p -bsf:v h264_mp4toannexb -f h264 testdata/clip-h264.h264

ls -l testdata/clip-vp8.ivf testdata/clip-h264.h264
