#!/usr/bin/env bash
# Build all 6 images, push to VCR.
#
# Usage:
#   IMAGE_TAG=v4 bash deploy/build_push.sh
#   BUILD_ONLY=manager IMAGE_TAG=v4 bash deploy/build_push.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

export DOCKER_BUILDKIT=1

REGISTRY="${VCR_REGISTRY:-vcr.vngcloud.vn/111480-abp111935}"
TAG="${IMAGE_TAG:-v1}"
PLATFORM="${BUILD_PLATFORM:-linux/amd64}"
CR_SCRIPT=".claude/skills/agentbase/scripts/cr.sh"
BUILD_CACHE="${BUILD_CACHE:-1}"
CACHE_DIR="${CACHE_DIR:-$ROOT/.buildcache}"
REGISTRY_CACHE_TAG="${REGISTRY_CACHE_TAG:-buildcache}"
BUILDX_BUILDER="${BUILDX_BUILDER:-agentsdast-build}"
BUILDX_READY=0
OPENAI_PLUGINS_REPO="${OPENAI_PLUGINS_REPO:-https://github.com/openai/plugins.git}"
OPENAI_PLUGINS_REF="${OPENAI_PLUGINS_REF:-main}"
OPENAI_PLUGINS_CACHE_DIR="${OPENAI_PLUGINS_CACHE_DIR:-$ROOT/AgentSAST/.plugin-cache/openai-plugins}"
BUILD_RETRIES="${BUILD_RETRIES:-3}"
PUSH_RETRIES="${PUSH_RETRIES:-3}"
RETRY_SLEEP_SEC="${RETRY_SLEEP_SEC:-6}"

ensure_docker() {
  if docker info >/dev/null 2>&1; then
    return 0
  fi
  if command -v podman >/dev/null 2>&1; then
    local state
    state="$(podman machine inspect --format '{{.State}}' podman-machine-default 2>/dev/null || true)"
    if [ "$state" = "stopped" ]; then
      echo ">>> Starting Podman machine..."
      podman machine start
      sleep 2
    fi
    if docker context ls --format '{{.Name}}' 2>/dev/null | grep -qx podman; then
      docker context use podman >/dev/null
    fi
    docker info >/dev/null 2>&1 && return 0
  fi
  echo "ERROR: Docker daemon not running." >&2
  exit 1
}

ensure_buildx() {
  [ "$BUILDX_READY" = "1" ] && return 0
  [ "$BUILD_CACHE" != "1" ] && return 1
  command -v docker >/dev/null 2>&1 && docker buildx version >/dev/null 2>&1 || return 1

  if ! docker buildx inspect "$BUILDX_BUILDER" >/dev/null 2>&1; then
    echo ">>> Creating buildx builder: ${BUILDX_BUILDER}"
    docker buildx create --name "$BUILDX_BUILDER" --driver docker-container --use >/dev/null
  else
    docker buildx use "$BUILDX_BUILDER" >/dev/null
  fi
  docker buildx inspect --bootstrap "$BUILDX_BUILDER" >/dev/null 2>&1 || return 1
  BUILDX_READY=1
}

warm_registry_cache() {
  local name="$1"
  docker pull "${REGISTRY}/${name}:${REGISTRY_CACHE_TAG}" >/dev/null 2>&1 || true
  docker pull "${REGISTRY}/${name}:${TAG}" >/dev/null 2>&1 || true
  [ -n "${CACHE_FROM_TAG:-}" ] && docker pull "${REGISTRY}/${name}:${CACHE_FROM_TAG}" >/dev/null 2>&1 || true
}

build_image() {
  local name="$1" ref="$2" dockerfile="$3"
  local -a cache_args=()

  if ensure_buildx; then
    local local_cache="${CACHE_DIR}/${name}"
    mkdir -p "$local_cache"
    warm_registry_cache "$name"
    cache_args+=(
      --cache-from "type=local,src=${local_cache}"
      --cache-to "type=local,dest=${local_cache},mode=max"
      --cache-from "type=registry,ref=${REGISTRY}/${name}:${REGISTRY_CACHE_TAG}"
      --cache-to "type=registry,ref=${REGISTRY}/${name}:${REGISTRY_CACHE_TAG},mode=max"
      --cache-from "type=registry,ref=${REGISTRY}/${name}:${TAG}"
    )
    [ -n "${CACHE_FROM_TAG:-}" ] && cache_args+=(--cache-from "type=registry,ref=${REGISTRY}/${name}:${CACHE_FROM_TAG}")
    local attempt
    for attempt in $(seq 1 "$BUILD_RETRIES"); do
      if docker buildx build --load --platform "${PLATFORM}" "${cache_args[@]}" -f "${dockerfile}" -t "${ref}" .; then
        return 0
      fi
      if [ "$attempt" -lt "$BUILD_RETRIES" ]; then
        echo ">>> WARN build failed for ${name} (attempt ${attempt}/${BUILD_RETRIES}), prune local cache and retry..."
        rm -rf "$local_cache"
        mkdir -p "$local_cache"
        sleep "$RETRY_SLEEP_SEC"
      fi
    done
    return 1
  fi

  if docker buildx version >/dev/null 2>&1; then
    local attempt
    for attempt in $(seq 1 "$BUILD_RETRIES"); do
      if docker buildx build --load --platform "${PLATFORM}" -f "${dockerfile}" -t "${ref}" .; then
        return 0
      fi
      if [ "$attempt" -lt "$BUILD_RETRIES" ]; then
        echo ">>> WARN buildx failed for ${name} (attempt ${attempt}/${BUILD_RETRIES}), retry..."
        sleep "$RETRY_SLEEP_SEC"
      fi
    done
    return 1
  else
    local attempt
    for attempt in $(seq 1 "$BUILD_RETRIES"); do
      if docker build --platform "${PLATFORM}" -f "${dockerfile}" -t "${ref}" .; then
        return 0
      fi
      if [ "$attempt" -lt "$BUILD_RETRIES" ]; then
        echo ">>> WARN docker build failed for ${name} (attempt ${attempt}/${BUILD_RETRIES}), retry..."
        sleep "$RETRY_SLEEP_SEC"
      fi
    done
    return 1
  fi
}

push_image() {
  local name="$1" ref="$2"
  local attempt
  for attempt in $(seq 1 "$PUSH_RETRIES"); do
    if docker push "$ref"; then
      return 0
    fi
    if [ "$attempt" -lt "$PUSH_RETRIES" ]; then
      echo ">>> WARN push failed for ${name} (attempt ${attempt}/${PUSH_RETRIES}), retry..."
      sleep "$RETRY_SLEEP_SEC"
    fi
  done
  return 1
}

should_build() {
  local name="$1"
  [ -z "${BUILD_ONLY:-}" ] && return 0
  local part
  IFS=',' read -ra parts <<< "${BUILD_ONLY}"
  for part in "${parts[@]}"; do
    [ "$part" = "$name" ] && return 0
  done
  return 1
}

sync_openai_plugins_cache() {
  echo "=== Sync openai/plugins cache ==="
  mkdir -p "$(dirname "$OPENAI_PLUGINS_CACHE_DIR")"
  if [ -d "$OPENAI_PLUGINS_CACHE_DIR/.git" ]; then
    echo ">>> Updating existing cache: $OPENAI_PLUGINS_CACHE_DIR"
    git -C "$OPENAI_PLUGINS_CACHE_DIR" fetch --depth=1 origin "$OPENAI_PLUGINS_REF"
    git -C "$OPENAI_PLUGINS_CACHE_DIR" checkout -q FETCH_HEAD
  else
    echo ">>> Cloning plugins repo to cache: $OPENAI_PLUGINS_CACHE_DIR"
    rm -rf "$OPENAI_PLUGINS_CACHE_DIR"
    git clone --depth=1 --branch "$OPENAI_PLUGINS_REF" "$OPENAI_PLUGINS_REPO" "$OPENAI_PLUGINS_CACHE_DIR"
  fi

  if [ ! -d "$OPENAI_PLUGINS_CACHE_DIR/plugins/codex-security" ]; then
    echo "ERROR: codex-security not found in cached openai/plugins repo" >&2
    exit 1
  fi
}

IMAGES=(
  "runtime-mysql:runtimes/mysql/Dockerfile"
  "runtime-minio:runtimes/minio/Dockerfile"
  "manager:Manager/Dockerfile"
  "agent-sast:AgentSAST/Dockerfile"
  "agent-dast:AgentDAST/Dockerfile"
  "agent-report:AgentReport/Dockerfile"
)

ensure_docker
echo "=== Build cache: $([ "$BUILD_CACHE" = "1" ] && echo "on ($CACHE_DIR)" || echo off) ==="

# agent-sast no longer bundles the codex-security plugin (Codex was removed in
# favor of a pure-Go chat-completions tool loop), so the openai/plugins cache is
# not needed at build time.

if ! docker pull "${REGISTRY}/manager:${TAG}" >/dev/null 2>&1; then
  bash "$CR_SCRIPT" credentials docker-login
fi

fail=0
for entry in "${IMAGES[@]}"; do
  name="${entry%%:*}"
  dockerfile="${entry#*:}"
  should_build "$name" || { echo ">>> SKIP ${name}"; continue; }
  ref="${REGISTRY}/${name}:${TAG}"
  echo ">>> BUILD ${name} -> ${ref}"
  build_image "$name" "$ref" "$dockerfile" || { echo "!!! BUILD FAILED: ${name}"; fail=1; continue; }
  push_image "$name" "$ref" || { echo "!!! PUSH FAILED: ${name}"; fail=1; continue; }
  echo ">>> DONE ${name}"
done

[ "$fail" -eq 0 ] || exit 1
echo "=== All images pushed (tag=${TAG}) ==="
