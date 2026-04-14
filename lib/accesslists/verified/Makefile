# Formal verification of access list membership logic.
#
# Targets:
#   rust-build         Build the Rust crate (release, with FFI)
#   rust-build-target  Build with explicit target triple (for CGO linking)
#   rust-test          Run Rust unit tests
#   go-test            Run Go equivalence tests (requires Rust build first)
#   aeneas-translate   Translate Rust to Lean4 via Aeneas
#   lean-verify        Check Lean4 proofs
#   all                Full pipeline: build, translate, verify

.PHONY: rust-build rust-build-target rust-test go-test aeneas-translate lean-verify all clean

# Detect the cargo target triple for the current platform.
UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S),Darwin)
  ifeq ($(UNAME_M),arm64)
    CARGO_TARGET := aarch64-apple-darwin
  else
    CARGO_TARGET := x86_64-apple-darwin
  endif
else
  ifeq ($(UNAME_M),x86_64)
    CARGO_TARGET := x86_64-unknown-linux-gnu
  else
    CARGO_TARGET := aarch64-unknown-linux-gnu
  endif
endif

REPO_ROOT := $(shell cd ../../.. && pwd)

rust-build:
	cargo build -p accesslist-verify --release

rust-build-target:
	cargo build -p accesslist-verify --release --target $(CARGO_TARGET)

rust-test:
	cargo test -p accesslist-verify

go-test: rust-build-target
	cd $(REPO_ROOT) && go test -tags verified_accesslists -count=1 -v ./lib/accesslists/verified/

aeneas-translate:
	./scripts/aeneas-translate.sh

lean-verify:
	./scripts/verify.sh

all: rust-test go-test lean-verify

clean:
	cargo clean -p accesslist-verify
	cd lean4 && lake clean 2>/dev/null || true
