#!/usr/bin/env sh
set -eu

if [ "$#" -lt 1 ] || [ "$#" -gt 3 ]; then
  echo "usage: $0 <url> [timeout_seconds] [name]" >&2
  exit 2
fi

URL="$1"
TIMEOUT="${2:-60}"
NAME="${3:-$URL}"

start_ts="$(date +%s)"

check_url() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsS --max-time 2 "$URL" >/dev/null
    return $?
  fi

  if command -v wget >/dev/null 2>&1; then
    wget -q --spider "$URL"
    return $?
  fi

  echo "error: neither curl nor wget is available" >&2
  return 127
}

while :; do
  if check_url; then
    echo "ready: $NAME"
    exit 0
  fi

  now_ts="$(date +%s)"
  elapsed=$((now_ts - start_ts))
  if [ "$elapsed" -ge "$TIMEOUT" ]; then
    echo "timeout: $NAME did not become ready within ${TIMEOUT}s ($URL)" >&2
    exit 1
  fi

  sleep 1
done
