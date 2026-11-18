#!/usr/bin/env bash
# Download CDnet 2014 and unpack selected sequences into data/cdnet/<category>/<sequence>/.
#
# Usage: scripts/download_cdnet.sh [category/sequence ...]
#   With no arguments the sequences used by the experiments are unpacked.
#
# CDnet is distributed as one large archive (several GB). It is downloaded
# once to data/cdnet/dataset2014.zip and kept, so picking other sequences
# later does not download it again. Set CDNET_URL to use a mirror, or put
# the archive there yourself and the download is skipped.
set -euo pipefail

CDNET_URL="${CDNET_URL:-http://jacarini.dinf.usherbrooke.ca/static/dataset/dataset2014.zip}"
DEST="${DEST:-data/cdnet}"
ZIP="$DEST/dataset2014.zip"

if [[ $# -gt 0 ]]; then
  sequences=("$@")
else
  sequences=(
    baseline/highway
    baseline/pedestrians
    shadow/peopleInShade
    shadow/busStation
    badWeather/skating
    badWeather/blizzard
    nightVideos/streetCornerAtNight
    lowFramerate/tramCrossroad_1fps
  )
fi

command -v unzip >/dev/null || { echo "unzip not found, please install it" >&2; exit 1; }
mkdir -p "$DEST"

if [[ ! -s "$ZIP" ]]; then
  command -v curl >/dev/null || { echo "curl not found, please install it" >&2; exit 1; }
  echo "Downloading CDnet 2014 from $CDNET_URL (large, resumable) ..."
  curl -fL --retry 3 -C - -o "$ZIP.part" "$CDNET_URL"
  mv "$ZIP.part" "$ZIP"
fi

for seq in "${sequences[@]}"; do
  if [[ -d "$DEST/$seq/input" ]]; then
    echo "already there: $seq"
    continue
  fi
  echo "unpacking $seq"
  # Archive entries are dataset/<category>/<sequence>/...; strip "dataset/".
  tmp="$(mktemp -d)"
  if ! unzip -q "$ZIP" "dataset/$seq/*" -d "$tmp"; then
    rm -rf "$tmp"
    echo "not found in the archive: $seq" >&2
    exit 1
  fi
  mkdir -p "$DEST/$(dirname "$seq")"
  mv "$tmp/dataset/$seq" "$DEST/$seq"
  rm -rf "$tmp"
done

echo "Done. List sequences with: heatmap list"
