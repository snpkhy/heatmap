#!/usr/bin/env bash
# Split a video into PNG frames with FFmpeg (I/O only).
# Usage: scripts/extract_frames.sh <video> <output_dir> [fps]
set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "Usage: $0 <video> <output_dir> [fps]" >&2
  exit 2
fi

video="$1"
out="$2"
fps="${3:-}"

command -v ffmpeg >/dev/null || { echo "ffmpeg not found, please install it" >&2; exit 1; }
[[ -f "$video" ]] || { echo "file not found: $video" >&2; exit 1; }

mkdir -p "$out"
filter=()
[[ -n "$fps" ]] && filter=(-vf "fps=$fps")

# CDnet-style names: in000001.png, in000002.png, ...
ffmpeg -hide_banner -loglevel error -i "$video" "${filter[@]}" "$out/in%06d.png"
echo "Frames written: $(find "$out" -name 'in*.png' | wc -l | tr -d ' ') -> $out"
