#!/usr/bin/env bash
set -Eeuo pipefail

BASE_URL="${1:-http://127.0.0.1:${GAMEPAGE_PORT:-8090}}"
MAX_ATTEMPTS="${MAX_ATTEMPTS:-60}"
SLEEP_SECONDS="${SLEEP_SECONDS:-2}"

retry() {
  local description="$1"
  shift

  local attempt
  for ((attempt = 1; attempt <= MAX_ATTEMPTS; attempt++)); do
    if "$@"; then
      printf 'PASS: %s\n' "$description"
      return 0
    fi
    sleep "$SLEEP_SECONDS"
  done

  printf 'FAIL: %s after %s attempts\n' "$description" "$MAX_ATTEMPTS" >&2
  return 1
}

http_ok() {
  curl --fail --silent --show-error --location --max-time 10 "$1" >/dev/null
}

body_contains() {
  local url="$1"
  local expected="$2"
  curl --fail --silent --show-error --location --max-time 10 "$url" | grep --fixed-strings --quiet "$expected"
}

printf 'Testing GamePage stack at %s\n' "$BASE_URL"

retry 'gateway liveness' http_ok "$BASE_URL/healthz"
retry 'complete stack readiness' http_ok "$BASE_URL/readyz"
retry 'COBOL-generated launcher' body_contains "$BASE_URL/" 'COBOL Game Mainframe'
retry 'Trump vs. Shakespeare route' http_ok "$BASE_URL/play/trump/"
retry 'Crazy Mini Golf route' http_ok "$BASE_URL/play/golf/"
retry 'Crazy Race route' http_ok "$BASE_URL/play/race/"
retry 'aggregated status is healthy' body_contains "$BASE_URL/api/status" '"overall":"ok"'

redirect_headers="$(curl --silent --show-error --head --max-time 10 "$BASE_URL/play/race")"
printf '%s\n' "$redirect_headers" | grep --extended-regexp --quiet '^HTTP/.* 308 '
printf '%s\n' "$redirect_headers" | grep --ignore-case --extended-regexp --quiet '^location: /play/race/'
printf 'PASS: canonical game-prefix redirect\n'

for service_port in \
  'trump-vs-shakespeare 8000' \
  'crazy-mini-golf 8080' \
  'crazy-race 8080'; do
  read -r service port <<<"$service_port"
  published="$(docker compose port "$service" "$port" 2>/dev/null || true)"
  if [[ -n "$published" ]]; then
    printf 'FAIL: %s unexpectedly publishes %s at %s\n' "$service" "$port" "$published" >&2
    exit 1
  fi
  printf 'PASS: %s:%s is private\n' "$service" "$port"
done

printf 'All Docker smoke tests passed.\n'
