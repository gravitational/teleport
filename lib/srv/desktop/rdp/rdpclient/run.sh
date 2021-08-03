#!/bin/bash
set -euo pipefail
cargo build && go build -tags desktop_access_beta testclient/main.go && ./main $@
