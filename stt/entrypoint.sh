#!/bin/sh
set -e

MODEL_PATH="/models/ggml-${WHISPER_MODEL:-small}.bin"

if [ ! -f "$MODEL_PATH" ]; then
    echo "Model not found: $MODEL_PATH"
    echo "Downloading ggml-${WHISPER_MODEL:-small}.bin..."
    curl -fsSL "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-${WHISPER_MODEL:-small}.bin" \
         -o "$MODEL_PATH"
fi

whisper-server \
    --model "$MODEL_PATH" \
    --host 0.0.0.0 \
    --port 8888 \
    --threads 4 \
    2>&1 | sh /log-pipe.sh
