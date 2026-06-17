#!/usr/bin/env bash
# Deploy stack to VNG vServer via SSH (sync files + docker compose up).
#
# Usage:
#   VSERVER_HOST=103.245.255.118 bash deploy/deploy.sh
#   REDEPLOY_SERVICE=manager IMAGE_TAG=v4 VSERVER_HOST=... bash deploy/deploy.sh
#
# Env: VSERVER_HOST (required), VSERVER_SSH_PORT (234), SSH_KEY (./vserver.pem),
#      IMAGE_TAG, GATEWAY_HOST (clawathon.cloud), GATEWAY_PORT (80), GATEWAY_HTTPS_PORT (443),
#      REDEPLOY_SERVICE, REMOTE_DIR (/opt/agentsdast)
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

HOST="${VSERVER_HOST:-}"
USER="${VSERVER_USER:-root}"
SSH_PORT="${VSERVER_SSH_PORT:-234}"
KEY="${SSH_KEY:-$ROOT/vserver.pem}"
REMOTE="${REMOTE_DIR:-/opt/agentsdast}"
TAG="${IMAGE_TAG:-v1}"
GATEWAY_HOST="${GATEWAY_HOST:-clawathon.cloud}"
GATEWAY_PORT="${GATEWAY_PORT:-80}"
GATEWAY_HTTPS_PORT="${GATEWAY_HTTPS_PORT:-443}"
REDEPLOY_SERVICE="${REDEPLOY_SERVICE:-}"

[ -n "$HOST" ] || { echo "ERROR: set VSERVER_HOST" >&2; exit 1; }
[ -f "$KEY" ] || { echo "ERROR: SSH key not found: $KEY" >&2; exit 1; }
[ -f .env ] || { echo "ERROR: .env missing in project root" >&2; exit 1; }
[ -f deploy/nginx/cert.pem ] && [ -f deploy/nginx/private-key.pem ] || {
  echo "ERROR: deploy/nginx/cert.pem and private-key.pem required for SSL" >&2
  exit 1
}

SSH=(ssh -i "$KEY" -p "$SSH_PORT" -o StrictHostKeyChecking=accept-new -o ConnectTimeout=15 "${USER}@${HOST}")
SCP=(scp -i "$KEY" -P "$SSH_PORT" -o StrictHostKeyChecking=accept-new)
RSYNC=(rsync -az -e "ssh -i $KEY -p $SSH_PORT -o StrictHostKeyChecking=accept-new")
chmod 600 "$KEY" 2>/dev/null || true

if [ -z "$REDEPLOY_SERVICE" ]; then
  echo "=== Install Docker ==="
  "${SCP[@]}" deploy/install-docker.sh "${USER}@${HOST}:/tmp/install-docker.sh"
  "${SSH[@]}" 'bash /tmp/install-docker.sh'
fi

echo "=== VCR login on vServer ==="
_script_dir="$ROOT/.claude/skills/agentbase/scripts"
# shellcheck source=/dev/null
source "$_script_dir/lib/config.sh"
# shellcheck source=/dev/null
source "$_script_dir/lib/common.sh"
repo_json=$(NO_PERSIST=1 REDACT_FIELDS="" api_call GET "${AGENTBASE_CR_URL}/repository" 3>&1 >/dev/null)
cred_json=$(NO_PERSIST=1 REDACT_FIELDS="" api_call GET "${AGENTBASE_CR_URL}/registry-credential" 3>&1 >/dev/null)
registry_url=$(echo "$repo_json" | jq -r '.registryUrl // empty')
cr_user=$(echo "$cred_json" | jq -r '.username // empty')
cr_secret=$(echo "$cred_json" | jq -r '.secret // empty')
REGISTRY="${registry_url}/$(echo "$repo_json" | jq -r '.name // empty')"
[ -n "$registry_url" ] && [ -n "$cr_user" ] && [ -n "$cr_secret" ] || {
  echo "ERROR: could not fetch VCR credentials" >&2
  exit 1
}
printf '%s' "$cr_secret" | "${SSH[@]}" "docker login '$registry_url' -u '$cr_user' --password-stdin"

echo "=== Sync files ==="
"${SSH[@]}" "mkdir -p '$REMOTE/nginx'"
"${RSYNC[@]}" docker-compose.vserver.yml deploy/remote-up.sh .env "${USER}@${HOST}:${REMOTE}/"
if [ -z "$REDEPLOY_SERVICE" ] || [ "$REDEPLOY_SERVICE" = "gateway" ] || [ "$REDEPLOY_SERVICE" = "runtime-minio" ]; then
  "${RSYNC[@]}" deploy/nginx/ "${USER}@${HOST}:${REMOTE}/nginx/"
fi
if [ -z "$REDEPLOY_SERVICE" ]; then
  "${RSYNC[@]}" skills/ "${USER}@${HOST}:${REMOTE}/skills/"
fi

PUBLIC_URL="https://${GATEWAY_HOST}"

"${SSH[@]}" "grep -q '^VCR_REGISTRY=' '$REMOTE/.env' 2>/dev/null || echo 'VCR_REGISTRY=$REGISTRY' >> '$REMOTE/.env'"
if [ -z "$REDEPLOY_SERVICE" ]; then
  if "${SSH[@]}" "grep -q '^IMAGE_TAG=' '$REMOTE/.env' 2>/dev/null"; then
    "${SSH[@]}" "sed -i 's|^IMAGE_TAG=.*|IMAGE_TAG=$TAG|' '$REMOTE/.env'"
  else
    "${SSH[@]}" "echo 'IMAGE_TAG=$TAG' >> '$REMOTE/.env'"
  fi
fi
"${SSH[@]}" "grep -q '^GATEWAY_HOST=' '$REMOTE/.env' 2>/dev/null || echo 'GATEWAY_HOST=$GATEWAY_HOST' >> '$REMOTE/.env'"
"${SSH[@]}" "sed -i 's|^GATEWAY_HOST=.*|GATEWAY_HOST=$GATEWAY_HOST|' '$REMOTE/.env' 2>/dev/null || true"
"${SSH[@]}" "grep -q '^GATEWAY_PORT=' '$REMOTE/.env' 2>/dev/null || echo 'GATEWAY_PORT=$GATEWAY_PORT' >> '$REMOTE/.env'"
"${SSH[@]}" "grep -q '^GATEWAY_HTTPS_PORT=' '$REMOTE/.env' 2>/dev/null || echo 'GATEWAY_HTTPS_PORT=$GATEWAY_HTTPS_PORT' >> '$REMOTE/.env'"
"${SSH[@]}" "grep -q '^MINIO_PUBLIC_URL=' '$REMOTE/.env' 2>/dev/null && sed -i \"s#^MINIO_PUBLIC_URL=.*#MINIO_PUBLIC_URL=${PUBLIC_URL}#\" '$REMOTE/.env' || echo 'MINIO_PUBLIC_URL=${PUBLIC_URL}' >> '$REMOTE/.env'"
"${SSH[@]}" "chmod +x '$REMOTE/remote-up.sh'"

echo "=== Start stack ==="
"${SSH[@]}" "cd '$REMOTE' && REDEPLOY_SERVICE='${REDEPLOY_SERVICE}' IMAGE_TAG='${TAG}' bash remote-up.sh"

echo "Deploy complete: ${PUBLIC_URL}/"
echo "SSH: ssh -i $KEY -p $SSH_PORT ${USER}@${HOST}"
