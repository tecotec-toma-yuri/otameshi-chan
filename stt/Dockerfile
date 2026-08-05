FROM python:3.11-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ffmpeg \
        curl \
    && rm -rf /var/lib/apt/lists/*

RUN pip install --no-cache-dir \
    faster-whisper \
    flask \
    gunicorn

COPY server.py /app/server.py
COPY entrypoint.sh /entrypoint.sh
COPY log-pipe.sh /log-pipe.sh
RUN chmod +x /entrypoint.sh /log-pipe.sh

RUN mkdir -p /models

ENV WHISPER_MODEL=base
ENV WHISPER_DEVICE=cpu
ENV WHISPER_COMPUTE_TYPE=int8

EXPOSE 8888

ENTRYPOINT ["/entrypoint.sh"]
