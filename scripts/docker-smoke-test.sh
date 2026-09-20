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
retry 'Movie Selector route' http_ok "$BASE_URL/play/movie-selector/"
retry 'aggregated status is healthy' body_contains "$BASE_URL/api/status" '"overall":"ok"'
retry 'COBOL owns application decisions' body_contains "$BASE_URL/api/status" '"decisionEngine":"gnucobol"'
retry 'minimum launchable policy is active' body_contains "$BASE_URL/api/status" '"minimumLaunchableGames":1'
retry 'healthy policy states are reported' body_contains "$BASE_URL/api/status" '"policyState":"healthy"'
retry 'all games are launchable' body_contains "$BASE_URL/api/status" '"launchable":true'
retry 'all games are required by default' body_contains "$BASE_URL/api/status" '"required":true'

for game_path_and_key in 'trump trump' 'golf golf' 'race race' 'movie-selector movies'; do
  read -r game_path favicon_key <<<"$game_path_and_key"
  retry "$game_path page has its branded favicon" body_contains "$BASE_URL/play/$game_path/" "data-gamepage-favicon=\"$favicon_key\""
  retry "$game_path favicon is self-contained" body_contains "$BASE_URL/play/$game_path/" 'href="data:image/svg+xml,'
  retry "$game_path page links back to the launcher" body_contains "$BASE_URL/play/$game_path/" 'class="gamepage-back-link"'
done

trump_page="$(curl --fail --silent --show-error --location --max-time 10 "$BASE_URL/play/trump/")"
golf_page="$(curl --fail --silent --show-error --location --max-time 10 "$BASE_URL/play/golf/")"
race_page="$(curl --fail --silent --show-error --location --max-time 10 "$BASE_URL/play/race/")"
movies_page="$(curl --fail --silent --show-error --location --max-time 10 "$BASE_URL/play/movie-selector/")"
trump_icon="$(grep --only-matching 'data-gamepage-favicon="trump"[^>]*' <<<"$trump_page" | head -n1)"
golf_icon="$(grep --only-matching 'data-gamepage-favicon="golf"[^>]*' <<<"$golf_page" | head -n1)"
race_icon="$(grep --only-matching 'data-gamepage-favicon="race"[^>]*' <<<"$race_page" | head -n1)"
movies_icon="$(grep --only-matching 'data-gamepage-favicon="movies"[^>]*' <<<"$movies_page" | head -n1)"
icons=("$trump_icon" "$golf_icon" "$race_icon" "$movies_icon")
if [[ "$(printf '%s\n' "${icons[@]}" | sort -u | wc -l)" -ne 4 ]]; then
  printf 'FAIL: game favicons are not distinct\n' >&2
  exit 1
fi
printf 'PASS: game favicons are distinct\n'

redirect_headers="$(curl --silent --show-error --head --max-time 10 "$BASE_URL/play/race")"
printf '%s\n' "$redirect_headers" | grep --extended-regexp --quiet '^HTTP/.* 308 '
printf '%s\n' "$redirect_headers" | grep --ignore-case --extended-regexp --quiet '^location: /play/race/'
printf 'PASS: canonical game-prefix redirect\n'

for service_port in 'trump-vs-shakespeare 8000' 'crazy-mini-golf 8080' 'crazy-race 8080' 'movie-selector 8080'; do
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

# Movie Selector alone gets outbound egress (to reach Wencke); the other three games
# must stay on the internal-only network with no route out, exactly as before.
movies_container="$(docker compose ps --quiet movie-selector)"
movies_networks="$(docker inspect "$movies_container" --format '{{json .NetworkSettings.Networks}}' | jq -r 'keys | sort | join(",")')"
if [[ "$movies_networks" != *"games-egress"* ]]; then
  printf 'FAIL: movie-selector is not attached to the egress network (networks: %s)\n' "$movies_networks" >&2
  exit 1
fi
printf 'PASS: movie-selector has egress; found on networks: %s\n' "$movies_networks"

for isolated_service in trump-vs-shakespeare crazy-mini-golf crazy-race; do
  container_id="$(docker compose ps --quiet "$isolated_service")"
  # Compose prefixes network names with the project name (e.g. game-page_games), so match the
  # logical `games` network by suffix and require it to be the only one.
  networks="$(docker inspect "$container_id" --format '{{json .NetworkSettings.Networks}}' | jq -r 'keys | sort | join(",")')"
  if [[ ! "$networks" =~ ^([A-Za-z0-9._-]+_)?games$ ]]; then
    printf 'FAIL: %s should be on only the internal games network, found: %s\n' "$isolated_service" "$networks" >&2
    exit 1
  fi
done
printf 'PASS: the other three games remain on the internal-only network with no egress\n'

printf 'All Docker smoke tests passed.\n'
