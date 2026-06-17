#!/usr/bin/env bash
# Run on vServer in /opt/agentsdast after sync.
set -euo pipefail
cd "$(dirname "$0")"

COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.vserver.yml}"
REDEPLOY_SERVICE="${REDEPLOY_SERVICE:-}"
VALID_SERVICES=(runtime-mysql runtime-minio manager gateway agent-sast agent-dast agent-report)

is_valid_service() {
  local name="$1" s
  for s in "${VALID_SERVICES[@]}"; do [ "$s" = "$name" ] && return 0; done
  return 1
}

[ -f .env ] || { echo "ERROR: .env missing" >&2; exit 1; }

REQUESTED_IMAGE_TAG="${IMAGE_TAG:-}"
set -a
# shellcheck disable=SC1091
source .env
set +a

: "${VCR_REGISTRY:?Set VCR_REGISTRY in .env}"
STACK_IMAGE_TAG="${IMAGE_TAG:-v1}"
IMAGE_TAG="${REQUESTED_IMAGE_TAG:-$STACK_IMAGE_TAG}"

redeploy_service() {
  local service="$1" tag="$2"
  local image_ref="${VCR_REGISTRY}/${service}:${tag}"
  echo ">>> Pull ${image_ref}"
  docker pull "$image_ref"
  echo ">>> Recreate ${service}"
  IMAGE_TAG="$tag" docker compose -f "$COMPOSE_FILE" up -d --no-deps --pull never --force-recreate "$service"
}

recreate_gateway() {
  docker compose -f "$COMPOSE_FILE" up -d --no-deps --force-recreate gateway
}

if [ -n "$REDEPLOY_SERVICE" ]; then
  is_valid_service "$REDEPLOY_SERVICE" || {
    echo "ERROR: invalid REDEPLOY_SERVICE='$REDEPLOY_SERVICE'" >&2
    exit 1
  }
  echo ">>> Redeploy ${REDEPLOY_SERVICE} (tag=${IMAGE_TAG})"
  if [ "$REDEPLOY_SERVICE" = "gateway" ]; then
    recreate_gateway
  else
    redeploy_service "$REDEPLOY_SERVICE" "$IMAGE_TAG"
    case "$REDEPLOY_SERVICE" in
      manager|agent-sast|agent-dast|agent-report) recreate_gateway ;;
    esac
  fi
  docker compose -f "$COMPOSE_FILE" ps
  exit 0
fi

echo ">>> Pull images (${IMAGE_TAG})"
docker compose -f "$COMPOSE_FILE" pull
echo ">>> Up stack"
docker compose -f "$COMPOSE_FILE" up -d --remove-orphans
recreate_gateway
docker compose -f "$COMPOSE_FILE" ps

GW_PORT="${GATEWAY_HTTPS_PORT:-443}"
GW_HOST="${GATEWAY_HOST:-clawathon.cloud}"
sleep 15
if curl -sfk "https://127.0.0.1/health" -H "Host: ${GW_HOST}" >/dev/null; then
  echo "Gateway /health OK (https://${GW_HOST})"
else
  echo "WARN: gateway health pending"
fi
