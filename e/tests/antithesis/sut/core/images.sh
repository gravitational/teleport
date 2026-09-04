#!/usr/bin/env bash

_output() {
  local key="$1"
  local value="$2"
  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "${key}=${value}" >> "$GITHUB_OUTPUT"
  fi
}

IMAGES=(
  "${REGISTRY:+${REGISTRY}/}antithesis/teleport:${TAG:-latest}"
  "${REGISTRY:+${REGISTRY}/}antithesis/auth:${TAG:-latest}"
  "${REGISTRY:+${REGISTRY}/}antithesis/pki:${TAG:-latest}"
  "${REGISTRY:+${REGISTRY}/}antithesis/pg:${TAG:-latest}"
  "${REGISTRY:+${REGISTRY}/}antithesis/nginx:${TAG:-latest}"
  "${REGISTRY:+${REGISTRY}/}antithesis/k3s-rancher:${TAG:-latest}"
  "${REGISTRY:+${REGISTRY}/}antithesis/workload-core:${TAG:-latest}"
)

CONFIG_IMAGE="${REGISTRY:+${REGISTRY}/}antithesis/config-core:${TAG:-latest}"

# Join the IMAGES array into a single semicolon-delimited string
printf -v IMAGES_STR '%s;' "${IMAGES[@]}"

# Strip the trailing ";"
IMAGES_STR="${IMAGES_STR%;}"

_output "images" "${IMAGES_STR}"
_output "config_image" "${CONFIG_IMAGE}"
