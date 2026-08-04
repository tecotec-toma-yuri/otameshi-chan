FROM golang:1.23-alpine

WORKDIR /app

RUN apk add --no-cache git ffmpeg

COPY . .
RUN go mod tidy
RUN chmod +x entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["./entrypoint.sh"]
CMD ["go", "run", "./cmd/server"]
