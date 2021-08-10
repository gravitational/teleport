#!/bin/bash
set -euo pipefail

cargo build
cargo install cbindgen
cbindgen --crate rdp-client --output librdprs.h --lang c

go build -tags desktop_access_beta testclient/main.go
./main $@
