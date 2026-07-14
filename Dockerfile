# syntax=docker/dockerfile:1.7

FROM debian:bookworm-slim AS cobol-builder
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get install -y --no-install-recommends gnucobol ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /src
COPY src/game_registry.cpy ./game_registry.cpy
COPY src/gamepage.cob ./gamepage.cob
COPY src/gamepage_registry.cob ./gamepage_registry.cob
COPY src/gamepage_core.cob ./gamepage_core.cob
RUN mkdir -p /out
RUN cobc -x -free -Wall -I . -o /usr/local/bin/gamepage-builder gamepage.cob \
    && cobc -x -free -Wall -I . -o /usr/local/bin/gamepage-registry-builder gamepage_registry.cob \
    && cobc -x -free -Wall -I . -o /out/gamepage-core gamepage_core.cob
RUN mkdir -p /out/assets /out/runtime \
    && cd /out \
    && /usr/local/bin/gamepage-builder \
    && /usr/local/bin/gamepage-registry-builder \
    && test -s /out/index.html \
    && test -s /out/assets/styles.css \
    && test -s /out/assets/status.js \
    && test -s /out/runtime/routes.tsv \
    && test -s /out/runtime/policy.tsv \
    && test -s /out/runtime/architecture.json \
    && grep -q "COBOL Game Mainframe" /out/index.html \
    && grep -q "Trump vs. Shakespeare" /out/index.html \
    && grep -q "TRUMP_ENABLED" /out/runtime/routes.tsv \
    && grep -q "MINIMUM_LAUNCHABLE_GAMES" /out/runtime/policy.tsv \
    && grep -q 'decision-engine-unavailable' /out/assets/status.js
COPY deploy/gamepage-nav.css /out/assets/gamepage-nav.css
RUN test -s /out/assets/gamepage-nav.css

FROM golang:1.26-bookworm AS go-builder
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go test ./... \
    && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/gateway ./cmd/gateway

FROM debian:bookworm-slim AS runtime
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update \
    && apt-get install -y --no-install-recommends libcob4 ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=go-builder --chown=65532:65532 /out/gateway /app/gateway
COPY --from=cobol-builder --chown=65532:65532 /out/gamepage-core /app/gamepage-core
COPY --from=cobol-builder --chown=65532:65532 /out /app/site
ENV LISTEN_ADDRESS=:8080 \
    SITE_DIRECTORY=/app/site \
    PUBLIC_ORIGIN=https://game.luisbenedikt.de \
    HEALTH_TIMEOUT=3s \
    COBOL_CORE_EXECUTABLE=/app/gamepage-core \
    COBOL_REGISTRY_PATH=/app/site/runtime/routes.tsv \
    COBOL_POLICY_PATH=/app/site/runtime/policy.tsv \
    MAINTENANCE_MODE=false
EXPOSE 8080
USER 65532:65532
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD ["/app/gateway", "healthcheck"]
ENTRYPOINT ["/app/gateway"]
