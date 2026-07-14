# syntax=docker/dockerfile:1.7

FROM debian:bookworm-slim AS cobol-builder
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get install -y --no-install-recommends gnucobol ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY src/gamepage.cob ./gamepage.cob
RUN cobc -x -free -Wall -o /usr/local/bin/gamepage-builder gamepage.cob
RUN mkdir -p /out/assets \
    && cd /out \
    && /usr/local/bin/gamepage-builder \
    && test -s /out/index.html \
    && test -s /out/assets/styles.css \
    && test -s /out/assets/status.js \
    && grep -q "COBOL Game Mainframe" /out/index.html

FROM golang:1.23-bookworm AS go-builder
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go test ./... \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app
COPY --from=go-builder --chown=65532:65532 /out/gateway /app/gateway
COPY --from=cobol-builder --chown=65532:65532 /out /app/site
ENV LISTEN_ADDRESS=:8080 \
    SITE_DIRECTORY=/app/site \
    PUBLIC_ORIGIN=https://game.luisbenedikt.de \
    HEALTH_TIMEOUT=3s
EXPOSE 8080
USER 65532:65532
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/gateway", "healthcheck"]
ENTRYPOINT ["/app/gateway"]
