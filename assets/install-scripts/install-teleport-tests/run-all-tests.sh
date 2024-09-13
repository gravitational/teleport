#!/bin/bash
set -e -o pipefail
docker compose up
if docker compose logs | grep "INSTALL_SCRIPT_TEST_FAILURE"; then 
    echo "ONE OR MORE TESTS FAILED"
    exit 1
fi
echo "ALL TESTS COMPLETED SUCCESSFULLY"
