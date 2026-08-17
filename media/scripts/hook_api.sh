#!/bin/sh
# Forward MediaMTX lifecycle events to the API for source paths only.
# Variant paths (*_360p / *_480p / *_720p / *_1080p) are ignored.

set -eu

EVENT="${1:?event required}"
API_BASE_URL="${API_BASE_URL:-http://api:5555}"

case "$MTX_PATH" in
  *_360p|*_480p|*_720p|*_1080p)
    exit 0
    ;;
esac

case "$EVENT" in
  not-ready)
    # Keep the last playlist on disk so clients can recover gracefully.
    # on_ready.sh replaces the ladder on the next publish.
    curl --fail --silent --show-error \
      --request POST \
      --data-urlencode "path=$MTX_PATH" \
      --data-urlencode "source_type=${MTX_SOURCE_TYPE:-}" \
      --data-urlencode "source_id=${MTX_SOURCE_ID:-}" \
      "$API_BASE_URL/mediamtx/hooks/not-ready"
    ;;
  read)
    curl --fail --silent --show-error \
      --request POST \
      --data-urlencode "path=$MTX_PATH" \
      --data-urlencode "reader_type=${MTX_READER_TYPE:-}" \
      --data-urlencode "reader_id=${MTX_READER_ID:-}" \
      "$API_BASE_URL/mediamtx/hooks/read"
    ;;
  unread)
    curl --fail --silent --show-error \
      --request POST \
      --data-urlencode "path=$MTX_PATH" \
      --data-urlencode "reader_type=${MTX_READER_TYPE:-}" \
      --data-urlencode "reader_id=${MTX_READER_ID:-}" \
      "$API_BASE_URL/mediamtx/hooks/unread"
    ;;
  *)
    echo "unknown hook event: $EVENT" >&2
    exit 1
    ;;
esac
