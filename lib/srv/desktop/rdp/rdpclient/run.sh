#!/bin/bash
set -euo pipefail

cargo build
cargo install cbindgen
cbindgen --crate rdp-client --output librdprs.h --lang c

rm main || true
go build -tags desktop_access_beta testclient/main.go
export RUST_BACKTRACE=1
export RUST_LOG=debug
./main "$@"
