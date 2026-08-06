# ビルドステージ
FROM golang:1.24-alpine AS builder

WORKDIR /app

# 依存関係のダウンロード
COPY go.mod go.sum ./
RUN go mod download

# ソースコードのコピーとビルド
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /server cmd/server/main.go

# 実行ステージ
FROM alpine:latest

WORKDIR /root/

# ビルド済みバイナリのコピー
COPY --from=builder /server .

EXPOSE 3000

CMD ["./server"]
