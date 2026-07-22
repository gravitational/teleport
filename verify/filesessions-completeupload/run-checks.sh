#!/usr/bin/env bash
#
# Model-checks CompleteUpload against a concurrent upload of the same session.
# Passes on this branch; fails on the pre-fix code. Expected: 0 bugs.
#
# Needs the P checker: dotnet tool install --global p
set -euo pipefail
cd "$(dirname "$0")"

p compile
p check -tc tcNoTornUpload -s "${SCHEDULES:-3000}"
