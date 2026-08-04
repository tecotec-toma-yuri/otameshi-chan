#!/bin/sh
# Reads lines from stdin, writes JSON to stdout and plain text to LOG_PATH.
# Usage: command 2>&1 | sh log-pipe.sh

LOG_PATH="${LOG_PATH:-logs/frontend.log}"
mkdir -p "$(dirname "$LOG_PATH")"

while IFS= read -r line; do
  ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  # stdout: JSON
  printf '{"time":"%s","level":"INFO","msg":"%s"}\n' "$ts" "$(echo "$line" | sed 's/"/\\"/g')"
  # file: plain text
  printf '%s %s\n' "$ts" "$line" >> "$LOG_PATH"
done
