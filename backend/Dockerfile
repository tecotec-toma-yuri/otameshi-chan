FROM golang:1.23-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o /server ./cmd/server

FROM alpine:3.20

RUN apk add --no-cache ffmpeg

WORKDIR /app
COPY --from=builder /server /app/server
COPY entrypoint.sh .
RUN chmod +x entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["./entrypoint.sh"]
CMD ["/app/server"]
