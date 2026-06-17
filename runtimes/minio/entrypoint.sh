#!/bin/sh
set -eu

minio server /data --address ":9000" --console-address ":9001" &
MINIO_PID=$!

echo "Waiting for MinIO..."
ready=0
for _ in $(seq 1 60); do
  if curl -sf http://127.0.0.1:9000/minio/health/live >/dev/null; then
    ready=1
    break
  fi
  if ! kill -0 "$MINIO_PID" 2>/dev/null; then
    echo "MinIO process exited before becoming ready"
    exit 1
  fi
  sleep 1
done

if [ "$ready" -ne 1 ]; then
  echo "MinIO did not become ready in time"
  exit 1
fi

if [ -n "${MINIO_BUCKET:-}" ]; then
  mc alias set local http://127.0.0.1:9000 "${MINIO_ROOT_USER}" "${MINIO_ROOT_PASSWORD}"
  mc mb "local/${MINIO_BUCKET}" --ignore-existing || true
  echo "Bucket ensured: ${MINIO_BUCKET}"
fi

echo "MinIO is ready (pid ${MINIO_PID})"

export AGENTBASE_HEALTH_PORT=8080
export HEALTH_CHECK_CMD="curl -sf http://127.0.0.1:9000/minio/health/live"
exec python3 /health_proxy.py
