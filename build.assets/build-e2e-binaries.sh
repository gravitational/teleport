#!/bin/bash
#
# Builds teleport, tctl, and tsh (with webauthnmock) in parallel for e2e tests.
# Build logs are written to build-logs/.
#

set -eo pipefail

MAKE="${MAKE:-make}"
BUILDDIR="${BUILDDIR:-build}"

cargo --version && rustc --version && echo "ARCH=$(uname -m)"

mkdir -p build-logs

${MAKE} "${BUILDDIR}/teleport" 2>&1 | tee build-logs/teleport.log & pid_teleport=$!
${MAKE} "${BUILDDIR}/tctl" 2>&1 | tee build-logs/tctl.log & pid_tctl=$!
go build -tags webauthnmock -o "${BUILDDIR}/tsh-e2e-webauthnmock" ./tool/tsh 2>&1 | tee build-logs/tsh.log & pid_tsh=$!

failed=0
wait $pid_teleport || { echo "::error::make ${BUILDDIR}/teleport failed with exit code $?"; failed=1; }
wait $pid_tctl     || { echo "::error::make ${BUILDDIR}/tctl failed with exit code $?"; failed=1; }
wait $pid_tsh      || { echo "::error::go build tsh failed with exit code $?"; failed=1; }
exit $failed
