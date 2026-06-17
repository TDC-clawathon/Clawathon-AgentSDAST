#!/bin/bash
set -euo pipefail

# Start MySQL using the official image entrypoint (background).
/usr/local/bin/docker-entrypoint.sh mysqld &
MYSQL_PID=$!

echo "Waiting for MySQL..."
ready=0
for _ in $(seq 1 90); do
  if mysqladmin ping -h 127.0.0.1 -uroot -p"${MYSQL_ROOT_PASSWORD}" --silent 2>/dev/null; then
    ready=1
    break
  fi
  if ! kill -0 "$MYSQL_PID" 2>/dev/null; then
    echo "MySQL process exited before becoming ready"
    exit 1
  fi
  sleep 2
done

if [ "$ready" -ne 1 ]; then
  echo "MySQL did not become ready in time"
  exit 1
fi

echo "MySQL is ready (pid ${MYSQL_PID})"

export AGENTBASE_HEALTH_PORT=8080
export HEALTH_CHECK_CMD="mysqladmin ping -h 127.0.0.1 -uroot -p${MYSQL_ROOT_PASSWORD} --silent"
exec python3 /health_proxy.py
