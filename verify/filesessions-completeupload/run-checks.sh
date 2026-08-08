#!/usr/bin/env bash
#
# Model-checks CompleteUpload against a concurrent upload of the same session.
# Reports the bug on this branch. Expected: 1 bug. Passes once the fix is applied.
#
# Needs the P checker: dotnet tool install --global p
set -uo pipefail
cd "$(dirname "$0")"

p compile
p check -tc tcNoTornUpload -s "${SCHEDULES:-3000}"
