#!/bin/sh
# Notify the API, then generate a live ABR HLS ladder with one master playlist.
# Toggle ladders with:
#   ABR_DISABLE_360P=1
#   ABR_DISABLE_480P=1
#   ABR_DISABLE_720P=1
#   ABR_DISABLE_1080P=1
#
# MediaMTX does not build multi-bitrate master playlists natively:
# https://github.com/bluenviron/mediamtx/issues/431

set -eu

RTMP_PORT="${RTMP_PORT:-1935}"

case "$MTX_PATH" in
  *_360p|*_480p|*_720p|*_1080p)
    exit 0
    ;;
esac

ENABLE_360P=1
ENABLE_480P=1
ENABLE_720P=1
ENABLE_1080P=1
[ "${ABR_DISABLE_360P:-0}" = "1" ] && ENABLE_360P=0
[ "${ABR_DISABLE_480P:-0}" = "1" ] && ENABLE_480P=0
[ "${ABR_DISABLE_720P:-0}" = "1" ] && ENABLE_720P=0
[ "${ABR_DISABLE_1080P:-0}" = "1" ] && ENABLE_1080P=0

COUNT=$((ENABLE_360P + ENABLE_480P + ENABLE_720P + ENABLE_1080P))
if [ "$COUNT" -eq 0 ]; then
  echo "all ABR ladders disabled; nothing to encode" >&2
  exit 1
fi

OUT="/hls/abr/${MTX_PATH}"
rm -rf "$OUT"
mkdir -p "$OUT"

wait_for_rtmp() {
  i=0
  while [ "$i" -lt 40 ]; do
    if ffprobe -v error -select_streams v:0 -show_entries stream=codec_type -of csv=p=0 \
      "rtmp://127.0.0.1:${RTMP_PORT}/${MTX_PATH}" 2>/dev/null | grep -q '^video$'; then
      return 0
    fi
    sleep 0.25
    i=$((i + 1))
  done
  echo "timed out waiting for RTMP stream ${MTX_PATH}" >&2
  return 1
}

INPUT_URL="rtmp://127.0.0.1:${RTMP_PORT}/${MTX_PATH}"
MAPS=""
FILTER_ARGS=""
VBITRATE_ARGS=""
ABITRATE_ARGS=""
VAR_MAP=""
STREAM_IDX=0

add_variant() {
  LABEL="$1"
  WIDTH="$2"
  HEIGHT="$3"
  MAXRATE="$4"
  ABITRATE="$5"

  mkdir -p "$OUT/v${LABEL}"
  MAPS="${MAPS} -map 0:v:0 -map 0:a:0"
  FILTER_ARGS="${FILTER_ARGS} -filter:v:${STREAM_IDX} scale=w=${WIDTH}:h=${HEIGHT}"
  VBITRATE_ARGS="${VBITRATE_ARGS} -maxrate:v:${STREAM_IDX} ${MAXRATE}"
  ABITRATE_ARGS="${ABITRATE_ARGS} -b:a:${STREAM_IDX} ${ABITRATE}"

  if [ -n "$VAR_MAP" ]; then
    VAR_MAP="${VAR_MAP} "
  fi
  VAR_MAP="${VAR_MAP}v:${STREAM_IDX},a:${STREAM_IDX},name:${LABEL}"
  STREAM_IDX=$((STREAM_IDX + 1))
}

if [ "$ENABLE_360P" = "1" ]; then
  add_variant 360p 480 360 600k 500k
fi
if [ "$ENABLE_480P" = "1" ]; then
  add_variant 480p 640 480 1500k 1000k
fi
if [ "$ENABLE_720P" = "1" ]; then
  add_variant 720p 1280 720 3000k 2000k
fi
if [ "$ENABLE_1080P" = "1" ]; then
  add_variant 1080p 1920 1080 5000k 2000k
fi

curl --fail --silent --show-error \
  --request POST \
  --data-urlencode "path=$MTX_PATH" \
  --data-urlencode "source_type=${MTX_SOURCE_TYPE:-}" \
  --data-urlencode "source_id=${MTX_SOURCE_ID:-}" \
  http://api:5555/mediamtx/hooks/ready

wait_for_rtmp

set -f
# shellcheck disable=SC2086
exec ffmpeg -hide_banner -loglevel warning \
  -f flv \
  -i "$INPUT_URL" \
  $MAPS \
  -c:v libx264 -crf 22 -preset fast \
  -c:a aac -ar 44100 \
  $FILTER_ARGS \
  $VBITRATE_ARGS \
  $ABITRATE_ARGS \
  -f hls \
  -hls_time 3 \
  -hls_list_size 10 \
  -hls_flags independent_segments \
  -master_pl_name master.m3u8 \
  -hls_segment_filename "${OUT}/v%v/seg_%05d.ts" \
  -var_stream_map "$VAR_MAP" \
  "${OUT}/v%v/index.m3u8"
