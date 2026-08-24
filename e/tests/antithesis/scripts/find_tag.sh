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


TAG="$(git -C "${REPO_PATH}" rev-parse --short HEAD)"

DIRTY=$(git -C "${REPO_PATH}"  status --porcelain -- . | grep -c '' || true)
if [[ "$DIRTY" -gt 0 ]]; then
  TAG="${TAG}-dirty"
fi

_output "tag" "$TAG"