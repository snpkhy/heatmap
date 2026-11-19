#!/usr/bin/env bash
# Download CDnet 2014 sequences into data/cdnet/<category>/<sequence>/.
#
# Usage: scripts/download_cdnet.sh [category/sequence ...]
#   With no arguments the sequences used by the experiments are downloaded.
#
# Every sequence is its own archive (16–220 MB), so only what is asked for is
# downloaded. Archives are kept in data/cdnet/.archives and reused; delete
# that folder to free the space. Set CDNET_URL to use a mirror with the same
# layout (<CDNET_URL>/<category>/<sequence>.zip).
set -euo pipefail

CDNET_URL="${CDNET_URL:-https://changedetection.net/static/dataset}"
DEST="${DEST:-data/cdnet}"
ARCHIVES="$DEST/.archives"

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
command -v curl >/dev/null || { echo "curl not found, please install it" >&2; exit 1; }
mkdir -p "$ARCHIVES"

for seq in "${sequences[@]}"; do
  if [[ ! "$seq" =~ ^[A-Za-z0-9_]+/[A-Za-z0-9_]+$ ]]; then
    echo "expected category/sequence, got: $seq" >&2
    exit 1
  fi
  if [[ -d "$DEST/$seq/input" ]]; then
    echo "already there: $seq"
    continue
  fi
  category="${seq%%/*}"
  name="${seq##*/}"
  zip="$ARCHIVES/$category-$name.zip"
  if [[ ! -s "$zip" ]]; then
    echo "downloading $seq"
    if ! curl -fL --retry 3 -C - -o "$zip.part" "$CDNET_URL/$seq.zip"; then
      echo "could not download $CDNET_URL/$seq.zip" >&2
      exit 1
    fi
    mv "$zip.part" "$zip"
  fi
  echo "unpacking $seq"
  # The archive holds <sequence>/input, <sequence>/groundtruth, …
  tmp="$(mktemp -d)"
  if ! unzip -q "$zip" -d "$tmp" || [[ ! -d "$tmp/$name/input" ]]; then
    rm -rf "$tmp"
    echo "unexpected archive layout: $zip" >&2
    exit 1
  fi
  mkdir -p "$DEST/$category"
  mv "$tmp/$name" "$DEST/$seq"
  rm -rf "$tmp"
done

echo "Done. List sequences with: heatmap list"
