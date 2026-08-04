FROM ubuntu:22.04 AS builder

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        build-essential \
        cmake \
        git \
        ca-certificates \
    && rm -rf /var/lib/apt/lists/*

RUN git clone --depth 1 https://github.com/ggerganov/whisper.cpp.git /whisper.cpp

WORKDIR /whisper.cpp

RUN cmake -B build -DWHISPER_BUILD_SERVER=ON -DBUILD_SHARED_LIBS=OFF && \
    cmake --build build --config Release -j$(nproc)

FROM ubuntu:22.04

RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        curl \
        ca-certificates \
        libgomp1 \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /whisper.cpp/build/bin/whisper-server /usr/local/bin/whisper-server

ARG WHISPER_MODEL=base
RUN mkdir -p /models && \
    curl -fsSL "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-${WHISPER_MODEL}.bin" \
         -o /models/ggml-${WHISPER_MODEL}.bin

ENV WHISPER_MODEL=base

EXPOSE 8888

COPY entrypoint.sh /entrypoint.sh
COPY log-pipe.sh /log-pipe.sh
RUN chmod +x /entrypoint.sh /log-pipe.sh

ENTRYPOINT ["/entrypoint.sh"]
