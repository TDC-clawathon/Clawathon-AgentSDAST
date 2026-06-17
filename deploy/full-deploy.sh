#!/usr/bin/env bash
# Build + push all images, then deploy to vServer.
#
# Usage:
#   VSERVER_HOST=103.245.255.118 bash deploy/full-deploy.sh
#   IMAGE_TAG=v4 VSERVER_HOST=... bash deploy/full-deploy.sh
#   REDEPLOY_SERVICE=manager IMAGE_TAG=v4 VSERVER_HOST=... bash deploy/full-deploy.sh
#   REDEPLOY_SERVICE=gateway VSERVER_HOST=... bash deploy/full-deploy.sh
#
# Env: VSERVER_HOST, IMAGE_TAG, SKIP_BUILD, BUILD_CACHE, CACHE_DIR, CACHE_FROM_TAG,
#      REDEPLOY_SERVICE, BUILD_ONLY, GATEWAY_PORT, SSH_KEY, VSERVER_SSH_PORT
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export IMAGE_TAG="${IMAGE_TAG:-v1}"
export BUILD_CACHE="${BUILD_CACHE:-1}"
export CACHE_DIR="${CACHE_DIR:-$ROOT/.buildcache}"
export REGISTRY_CACHE_TAG="${REGISTRY_CACHE_TAG:-buildcache}"
[ "${AUTO_TAG:-0}" = "1" ] && export IMAGE_TAG="v$(date +%Y%m%d%H%M%S)"

REDEPLOY_SERVICE="${REDEPLOY_SERVICE:-}"
if [ -n "$REDEPLOY_SERVICE" ] && [ -z "${BUILD_ONLY:-}" ] && [ "$REDEPLOY_SERVICE" != "gateway" ]; then
  export BUILD_ONLY="$REDEPLOY_SERVICE"
fi
[ "$REDEPLOY_SERVICE" = "gateway" ] && export SKIP_BUILD=1

HOST="${VSERVER_HOST:-}"
SSH_PORT="${VSERVER_SSH_PORT:-234}"
KEY="${SSH_KEY:-$ROOT/vserver.pem}"
GATEWAY_HOST="${GATEWAY_HOST:-clawathon.cloud}"
GATEWAY_HTTPS_PORT="${GATEWAY_HTTPS_PORT:-443}"

[ -n "$HOST" ] || { echo "ERROR: set VSERVER_HOST" >&2; exit 1; }

echo "=============================================="
[ -n "$REDEPLOY_SERVICE" ] && echo " Redeploy: $REDEPLOY_SERVICE" || echo " Full deploy"
echo "  host:  $HOST"
echo "  url:   https://${GATEWAY_HOST}"
echo "  tag:   $IMAGE_TAG"
echo "  cache: ${BUILD_CACHE}"
echo "=============================================="

if [ "${SKIP_BUILD:-0}" != "1" ]; then
  bash deploy/build_push.sh
else
  echo ">>> SKIP_BUILD=1"
fi

export VSERVER_HOST="$HOST" VSERVER_SSH_PORT="$SSH_PORT" SSH_KEY="$KEY"
export GATEWAY_HOST GATEWAY_HTTPS_PORT REDEPLOY_SERVICE
bash deploy/deploy.sh

SSH=(ssh -i "$KEY" -p "$SSH_PORT" -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15 "root@${HOST}")
if "${SSH[@]}" "curl -sfk https://127.0.0.1/health -H 'Host: ${GATEWAY_HOST}'" >/dev/null 2>&1; then
  echo "Health check: OK"
else
  echo "WARN: health check pending"
fi

echo "Done: https://${GATEWAY_HOST}/"
