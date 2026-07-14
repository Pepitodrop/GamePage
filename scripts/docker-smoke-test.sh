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

http_ok() { curl --fail --silent --show-error --location --max-time 10 "$1" >/dev/null; }
body_contains() {
  local url="$1" expected="$2" body
  body="$(curl --fail --silent --show-error --location --max-time 10 "$url")" || return 1
  grep --fixed-strings --quiet "$expected" <<<"$body"
}

printf 'Testing GamePage stack at %s\n' "$BASE_URL"
retry 'gateway liveness' http_ok "$BASE_URL/healthz"
retry 'complete stack readiness' http_ok "$BASE_URL/readyz"
retry 'registry-generated launcher' body_contains "$BASE_URL/" 'GAME_REGISTRY.CPY'
retry 'Trump vs. Shakespeare route' http_ok "$BASE_URL/play/trump/"
retry 'Crazy Mini Golf route' http_ok "$BASE_URL/play/golf/"
retry 'Crazy Race route' http_ok "$BASE_URL/play/race/"
retry 'aggregated status is healthy' body_contains "$BASE_URL/api/status" '"overall":"ok"'
retry 'COBOL owns application decisions' body_contains "$BASE_URL/api/status" '"decisionEngine":"gnucobol"'
retry 'minimum launchable policy is active' body_contains "$BASE_URL/api/status" '"minimumLaunchableGames":1'
retry 'healthy policy states are reported' body_contains "$BASE_URL/api/status" '"policyState":"healthy"'
retry 'all games are launchable' body_contains "$BASE_URL/api/status" '"launchable":true'
retry 'all games are required by default' body_contains "$BASE_URL/api/status" '"required":true'

for game_path in trump golf race; do
  retry "$game_path page has its branded favicon" body_contains "$BASE_URL/play/$game_path/" "data-gamepage-favicon=\"$game_path\""
  retry "$game_path favicon is self-contained" body_contains "$BASE_URL/play/$game_path/" 'href="data:image/svg+xml,'
  retry "$game_path page links back to the launcher" body_contains "$BASE_URL/play/$game_path/" 'class="gamepage-back-link"'
done

trump_page="$(curl --fail --silent --show-error --location --max-time 10 "$BASE_URL/play/trump/")"
golf_page="$(curl --fail --silent --show-error --location --max-time 10 "$BASE_URL/play/golf/")"
race_page="$(curl --fail --silent --show-error --location --max-time 10 "$BASE_URL/play/race/")"
trump_icon="$(grep --only-matching 'data-gamepage-favicon="trump"[^>]*' <<<"$trump_page" | head -n1)"
golf_icon="$(grep --only-matching 'data-gamepage-favicon="golf"[^>]*' <<<"$golf_page" | head -n1)"
race_icon="$(grep --only-matching 'data-gamepage-favicon="race"[^>]*' <<<"$race_page" | head -n1)"
if [[ "$trump_icon" == "$golf_icon" || "$trump_icon" == "$race_icon" || "$golf_icon" == "$race_icon" ]]; then
  printf 'FAIL: game favicons are not distinct\n' >&2
  exit 1
fi
printf 'PASS: game favicons are distinct\n'

redirect_headers="$(curl --silent --show-error --head --max-time 10 "$BASE_URL/play/race")"
printf '%s\n' "$redirect_headers" | grep --extended-regexp --quiet '^HTTP/.* 308 '
printf '%s\n' "$redirect_headers" | grep --ignore-case --extended-regexp --quiet '^location: /play/race/'
printf 'PASS: canonical game-prefix redirect\n'

for service_port in 'trump-vs-shakespeare 8000' 'crazy-mini-golf 8080' 'crazy-race 8080'; do
  read -r service port <<<"$service_port"
  container_id="$(docker compose ps --quiet "$service")"
  if [[ -z "$container_id" ]]; then
    printf 'FAIL: %s container is not running\n' "$service" >&2
    exit 1
  fi
  if ! docker inspect "$container_id" --format '{{json .NetworkSettings.Ports}}' \
      | jq --exit-status --arg port "$port/tcp" '(.[$port] // null) == null' >/dev/null; then
    printf 'FAIL: %s unexpectedly publishes %s\n' "$service" "$port" >&2
    docker inspect "$container_id" --format '{{json .NetworkSettings.Ports}}' | jq >&2
    exit 1
  fi
  printf 'PASS: %s:%s is private\n' "$service" "$port"
done
printf 'All Docker smoke tests passed.\n'
