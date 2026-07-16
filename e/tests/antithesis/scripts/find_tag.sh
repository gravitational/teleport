#!/usr/bin/env bash
set -euo pipefail

_repo_toplevel() {
  local toplevel
  toplevel="$(git rev-parse --show-toplevel)"
  # In a submodule, .git is a file rather than a directory
  if [[ -f "${toplevel}/.git" ]]; then
    realpath "$(git -C "${toplevel}" rev-parse --show-superproject-working-tree)"
  else
    realpath "${toplevel}"
  fi
}

_output() {
  local key="$1"
  local value="$2"
  if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
    echo "${key}=${value}" >> "$GITHUB_OUTPUT"
  else
    echo "${value}"
  fi
}

REPO_PATH="${REPO_PATH:-$(_repo_toplevel)}"
REPO_PATH="$(realpath "${REPO_PATH}")"
SUBMODULE_PATH="${SUBMODULE_PATH:-e}"

# Resolve submodule to absolute path
if [[ "${SUBMODULE_PATH}" != /* ]]; then
  SUBMODULE_ABS="${REPO_PATH}/${SUBMODULE_PATH}"
else
  SUBMODULE_ABS="${SUBMODULE_PATH}"
fi

TAG="$(git -C "${REPO_PATH}" rev-parse --short HEAD)"
OSS_SUB_MOVED=$(git -C "${REPO_PATH}" status --porcelain -- "${SUBMODULE_PATH}" | grep -c '' || true)
if [[ "$OSS_SUB_MOVED" -gt 0 ]]; then
	TAG="${TAG}-$(git -C "${SUBMODULE_ABS}" rev-parse --short HEAD)"
fi

OSS_DIRTY=$(git -C "${REPO_PATH}"  status --porcelain -- . ":(exclude)${SUBMODULE_PATH}" | grep -c '' || true)
SUB_INTERNAL_DIRTY=$(git -C "${SUBMODULE_ABS}" status --porcelain | grep -c '' || true)
if [[ "$OSS_DIRTY" -gt 0 || "$SUB_INTERNAL_DIRTY" -gt 0 ]]; then
  TAG="${TAG}-dirty"
fi

_output "tag" "$TAG"