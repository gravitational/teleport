#!/bin/sh
# Regenerate the Kubernetes access corpus from Go, build the Lean model,
# and run the Lean differential check. Exit non-zero on any mismatch.
set -eu
cd "$(dirname "$0")/.."   # <teleport>/lean/
(cd .. && go run ./tool/kubeaccess-corpus) > corpus/kubernetes.json
lake build
lake exe teleport-lean-diff corpus/kubernetes.json
