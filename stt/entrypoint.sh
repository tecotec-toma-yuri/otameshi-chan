#!/bin/sh
set -e

echo "Starting faster-whisper server (model=${WHISPER_MODEL:-small}, device=${WHISPER_DEVICE:-cpu}, compute_type=${WHISPER_COMPUTE_TYPE:-int8})"

exec gunicorn \
    --bind 0.0.0.0:8888 \
    --workers 1 \
    --threads 4 \
    --timeout 120 \
    "server:app" \
    --chdir /app \
    2>&1 | sh /log-pipe.sh
