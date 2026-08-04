FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && \
    apt-get install -y --no-install-recommends curl && \
    pip install --no-cache-dir "piper-plus[http]" python-json-logger && \
    python -c "import nltk; nltk.download('averaged_perceptron_tagger_eng', download_dir='/usr/local/nltk_data')" && \
    apt-get clean && rm -rf /var/lib/apt/lists/*

# つくよみちゃん（従来 WavLM 版）
RUN mkdir -p /app/models/tsukuyomi && \
    curl -fsSL "https://huggingface.co/ayousanz/piper-plus-tsukuyomi-chan/resolve/778b68b00722cd0239db5d79154dc6dcaa4a9b73/tsukuyomi-wavlm-300epoch.onnx" \
         -o /app/models/tsukuyomi/model.onnx && \
    curl -fsSL "https://huggingface.co/ayousanz/piper-plus-tsukuyomi-chan/resolve/778b68b00722cd0239db5d79154dc6dcaa4a9b73/config.json" \
         -o /app/models/tsukuyomi/config.json

# つくよみちゃん MB-iSTFT 版（現行 main）
RUN mkdir -p /app/models/tsukuyomi_mb && \
    curl -fsSL "https://huggingface.co/ayousanz/piper-plus-tsukuyomi-chan/resolve/main/tsukuyomi-chan-6lang-fp16.onnx" \
         -o /app/models/tsukuyomi_mb/model.onnx && \
    curl -fsSL "https://huggingface.co/ayousanz/piper-plus-tsukuyomi-chan/resolve/main/config.json" \
         -o /app/models/tsukuyomi_mb/config.json

# CSS10 Japanese
RUN mkdir -p /app/models/css10 && \
    curl -fsSL "https://huggingface.co/ayousanz/piper-plus-css10-ja-6lang/resolve/main/css10-ja-6lang-fp16.onnx" \
         -o /app/models/css10/model.onnx && \
    curl -fsSL "https://huggingface.co/ayousanz/piper-plus-css10-ja-6lang/resolve/main/config.json" \
         -o /app/models/css10/config.json

# 6言語ベース（多話者）
# 公開 HF の model.ckpt は emb_g 除去済みのため、推論用 ONNX が置かれた場合のみ有効化。
# 優先順: ローカル models/base_6lang → HF の候補ファイル名
RUN mkdir -p /app/models/base_6lang
COPY models/ /tmp/tts_models/
RUN set -e; \
    if [ -f /tmp/tts_models/base_6lang/model.onnx ] && [ -f /tmp/tts_models/base_6lang/config.json ]; then \
      cp -f /tmp/tts_models/base_6lang/model.onnx /app/models/base_6lang/model.onnx; \
      cp -f /tmp/tts_models/base_6lang/config.json /app/models/base_6lang/config.json; \
      echo "base_6lang: using local models/base_6lang"; \
    else \
      for f in \
        multilingual-6lang-mb-istft-scratch-75epoch.onnx \
        multilingual-6lang-fp16.onnx \
        model.onnx \
      ; do \
        if curl -fsSL "https://huggingface.co/ayousanz/piper-plus-base/resolve/main/${f}" \
             -o /app/models/base_6lang/model.onnx; then \
          curl -fsSL "https://huggingface.co/ayousanz/piper-plus-base/resolve/main/config.json" \
               -o /app/models/base_6lang/config.json || true; \
          if [ -f /app/models/base_6lang/config.json ]; then \
            echo "base_6lang: downloaded ${f}"; \
            break; \
          fi; \
        fi; \
        rm -f /app/models/base_6lang/model.onnx; \
      done; \
      if [ ! -f /app/models/base_6lang/model.onnx ]; then \
        echo "base_6lang: ONNX not available yet (skip)"; \
        rm -rf /app/models/base_6lang; \
      fi; \
    fi; \
    rm -rf /tmp/tts_models

COPY server.py .
COPY speakers_base_6lang.json /app/speakers_base_6lang.json

EXPOSE 5000

CMD ["python", "server.py"]
