#!/usr/bin/env bash
set -euo pipefail

# Antithesis smoke test for Kubernetes access through the CLI.

KUBE_WORKLOAD_HOME="$(mktemp -d)"
export KUBE_WORKLOAD_HOME

source "$(dirname "${BASH_SOURCE[0]}")/helper_kube_common.sh"

trap 'kube_cleanup_home' EXIT
kube_driver_init

kube_retry kctl_cluster get nodes -o wide >&2
