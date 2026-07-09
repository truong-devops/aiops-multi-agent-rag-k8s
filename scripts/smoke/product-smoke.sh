#!/usr/bin/env bash
set -euo pipefail

API_BASE="${API_BASE:-http://localhost:8080}"
RUN_ID="${RUN_ID:-$(date +%s)-${RANDOM:-0}}"

require_cmd() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required command: $1" >&2
    exit 1
  fi
}

log() {
  printf '[smoke] %s\n' "$*"
}

wait_url() {
  url="$1"
  name="$2"
  attempts="${3:-60}"
  i=1
  while [ "$i" -le "$attempts" ]; do
    if curl -fsS "$url" >/dev/null 2>&1; then
      log "$name is ready"
      return 0
    fi
    sleep 1
    i=$((i + 1))
  done
  echo "$name did not become ready: $url" >&2
  return 1
}

post_json() {
  path="$1"
  payload="$2"
  token="${3:-}"
  if [ -n "$token" ]; then
    curl -fsS -X POST "$API_BASE$path" \
      -H "Authorization: Bearer $token" \
      -H "Content-Type: application/json" \
      -d "$payload"
  else
    curl -fsS -X POST "$API_BASE$path" \
      -H "Content-Type: application/json" \
      -d "$payload"
  fi
}

put_json() {
  path="$1"
  payload="$2"
  token="$3"
  curl -fsS -X PUT "$API_BASE$path" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    -d "$payload"
}

delete_json() {
  path="$1"
  token="$2"
  curl -fsS -X DELETE "$API_BASE$path" \
    -H "Authorization: Bearer $token"
}

get_json() {
  path="$1"
  token="${2:-}"
  if [ -n "$token" ]; then
    curl -fsS "$API_BASE$path" -H "Authorization: Bearer $token"
  else
    curl -fsS "$API_BASE$path"
  fi
}

register_user() {
  suffix="$1"
  email="smoke-${RUN_ID}-${suffix}@example.com"
  username="smoke${RUN_ID//[^a-zA-Z0-9]/}${suffix}"
  payload="$(jq -nc \
    --arg email "$email" \
    --arg username "$username" \
    --arg display_name "Smoke User $suffix" \
    --arg password "StrongPass123!" \
    '{email:$email, username:$username, display_name:$display_name, password:$password}')"
  response="$(post_json "/api/v1/auth/register" "$payload")"
  user_id="$(printf '%s' "$response" | jq -r '.data.user.id')"
  if [ -z "$user_id" ] || [ "$user_id" = "null" ]; then
    echo "register did not return user id" >&2
    exit 1
  fi

  login_payload="$(jq -nc --arg email "$email" --arg password "StrongPass123!" '{email:$email, password:$password}')"
  login_response="$(post_json "/api/v1/auth/login" "$login_payload")"
  token="$(printf '%s' "$login_response" | jq -r '.data.access_token')"
  if [ -z "$token" ] || [ "$token" = "null" ]; then
    echo "login did not return access token" >&2
    exit 1
  fi
  printf '%s %s\n' "$user_id" "$token"
}

require_cmd curl
require_cmd jq

log "checking readiness"
wait_url "$API_BASE/readyz" "api-gateway"
wait_url "http://localhost:8081/readyz" "identity-service"
wait_url "http://localhost:8082/readyz" "video-service"
wait_url "http://localhost:8083/readyz" "feed-social-service"
wait_url "http://localhost:8084/readyz" "live-service"
wait_url "http://localhost:8086/readyz" "media-worker"

log "registering smoke users"
read -r user_id token < <(register_user "a")
read -r target_user_id target_token < <(register_user "b")
if [ -z "$target_token" ]; then
  echo "target token was not created" >&2
  exit 1
fi

log "testing live session API"
live_payload="$(jq -nc '{title:"Smoke live", description:"product smoke"}')"
live_response="$(post_json "/api/v1/live-sessions" "$live_payload" "$token")"
live_id="$(printf '%s' "$live_response" | jq -r '.data.id')"
stream_key="$(printf '%s' "$live_response" | jq -r '.data.stream_key')"
test -n "$live_id" && test "$live_id" != "null"
test -n "$stream_key" && test "$stream_key" != "null"
post_json "/api/v1/live-sessions/$live_id/start" '{}' "$token" | jq -e '.data.status == "live"' >/dev/null
post_json "/api/v1/live-sessions/$live_id/end" '{}' "$token" | jq -e '.data.status == "ended"' >/dev/null
get_json "/api/v1/live-sessions/$live_id" "$token" | jq -e '.data.stream_key == null and .data.status == "ended"' >/dev/null

log "testing video upload intent and confirmation"
video_payload="$(jq -nc '{title:"Smoke video", description:"product smoke", content_type:"video/mp4", size_bytes:19, visibility:"public"}')"
upload_response="$(curl -fsS -X POST "$API_BASE/api/v1/videos/upload-requests" \
  -H "Authorization: Bearer $token" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: smoke-$RUN_ID" \
  -d "$video_payload")"
video_id="$(printf '%s' "$upload_response" | jq -r '.data.video.id')"
upload_request_id="$(printf '%s' "$upload_response" | jq -r '.data.upload_request.id')"
upload_url="$(printf '%s' "$upload_response" | jq -r '.data.upload_request.upload_url')"
test -n "$video_id" && test "$video_id" != "null"
test -n "$upload_request_id" && test "$upload_request_id" != "null"
test -n "$upload_url" && test "$upload_url" != "null"

tmp_file="$(mktemp)"
trap 'rm -f "$tmp_file"' EXIT
printf 'aiops-smoke-video\n' > "$tmp_file"
curl -fsS -X PUT "$upload_url" -H "Content-Type: video/mp4" --data-binary "@$tmp_file" >/dev/null
confirm_payload="$(jq -nc --arg upload_request_id "$upload_request_id" '{upload_request_id:$upload_request_id, size_bytes:19}')"
post_json "/api/v1/videos/$video_id/uploaded" "$confirm_payload" "$token" | jq -e '.data.video.status == "uploaded"' >/dev/null
get_json "/api/v1/videos/$video_id" "$token" | jq -e --arg video_id "$video_id" '.data.video.id == $video_id' >/dev/null

log "waiting for worker ready event and feed ingestion"
ready=0
for _ in $(seq 1 45); do
  video_status="$(get_json "/api/v1/videos/$video_id" "$token" | jq -r '.data.video.status')"
  if [ "$video_status" = "ready" ]; then
    ready=1
    break
  fi
  sleep 1
done
if [ "$ready" != "1" ]; then
  echo "video did not become ready: $video_id" >&2
  exit 1
fi

in_feed=0
for _ in $(seq 1 45); do
  if get_json "/api/v1/feed" | jq -e --arg video_id "$video_id" '.data[] | select(.video_id == $video_id)' >/dev/null; then
    in_feed=1
    break
  fi
  sleep 1
done
if [ "$in_feed" != "1" ]; then
  echo "ready video did not appear in feed: $video_id" >&2
  exit 1
fi

log "testing social APIs through gateway"
put_json "/api/v1/videos/$video_id/like" '{}' "$target_token" | jq -e '.liked == true and .data.like_count >= 1' >/dev/null
get_json "/api/v1/videos/$video_id/social" "$target_token" | jq -e '.data.like_count >= 1' >/dev/null
comment_payload="$(jq -nc '{body:"hello from smoke test"}')"
comment_response="$(post_json "/api/v1/videos/$video_id/comments" "$comment_payload" "$target_token")"
comment_id="$(printf '%s' "$comment_response" | jq -r '.data.id')"
test -n "$comment_id" && test "$comment_id" != "null"
get_json "/api/v1/videos/$video_id/comments" "$target_token" | jq -e --arg comment_id "$comment_id" '.data[] | select(.id == $comment_id)' >/dev/null
delete_json "/api/v1/comments/$comment_id" "$target_token" | jq -e '.deleted == true' >/dev/null
put_json "/api/v1/users/$user_id/follow" '{}' "$target_token" | jq -e '.following == true' >/dev/null
delete_json "/api/v1/users/$user_id/follow" "$target_token" | jq -e '.following == false' >/dev/null

log "product backend smoke test passed"
