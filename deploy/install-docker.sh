#!/usr/bin/env bash
# Install Docker Engine + Compose plugin on Ubuntu vServer.
set -euo pipefail

run_as_root() {
  [ "$(id -u)" -eq 0 ] && "$@" || sudo "$@"
}

if command -v docker >/dev/null 2>&1; then
  echo "Docker already installed: $(docker --version)"
  exit 0
fi

export DEBIAN_FRONTEND=noninteractive
run_as_root apt-get update -qq
run_as_root apt-get install -y -qq ca-certificates curl gnupg
run_as_root install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | run_as_root gpg --dearmor -o /etc/apt/keyrings/docker.gpg
run_as_root chmod a+r /etc/apt/keyrings/docker.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" \
  | run_as_root tee /etc/apt/sources.list.d/docker.list >/dev/null
run_as_root apt-get update -qq
run_as_root apt-get install -y -qq docker-ce docker-ce-cli containerd.io docker-compose-plugin
echo "Docker installed."
