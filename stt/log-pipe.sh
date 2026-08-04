#!/bin/sh
# Reads lines from stdin, writes JSON to stdout and plain text to LOG_PATH.

LOG_PATH="${LOG_PATH:-logs/stt.log}"
mkdir -p "$(dirname "$LOG_PATH")"

while IFS= read -r line; do
  ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  printf '{"time":"%s","level":"INFO","msg":"%s"}\n' "$ts" "$(echo "$line" | sed 's/"/\\"/g')"
  printf '%s %s\n' "$ts" "$line" >> "$LOG_PATH"
done
