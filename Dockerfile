FROM golang:1.26.1-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/downloader ./cmd/downloader && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/ingestor ./cmd/ingestor && \
    CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache bash ca-certificates tzdata curl

COPY --from=builder /out/downloader /usr/local/bin/downloader
COPY --from=builder /out/ingestor /usr/local/bin/ingestor
COPY --from=builder /out/api /usr/local/bin/api
COPY scripts/daily_download_ingest.sh /usr/local/bin/daily_download_ingest.sh
COPY scripts/monitor_snapshot.sh /usr/local/bin/monitor_snapshot.sh
COPY docker/start-cron.sh /usr/local/bin/start-cron.sh

RUN chmod +x /usr/local/bin/downloader /usr/local/bin/ingestor /usr/local/bin/api \
    /usr/local/bin/daily_download_ingest.sh /usr/local/bin/monitor_snapshot.sh /usr/local/bin/start-cron.sh

ENV TZ=Asia/Tokyo \
    DATA_ROOT=/data

EXPOSE 8080

ENTRYPOINT ["/bin/bash", "-lc"]
CMD ["daily_download_ingest.sh"]
