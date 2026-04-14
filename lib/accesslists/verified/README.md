# Formal Verification of Access List Membership Logic

This directory contains a proof of concept for formally verifying Teleport's
access list membership checking logic using Lean4.

## Architecture

```
                    ┌─────────────┐
                    │   Go code   │
                    │ (caller)    │
                    └──────┬──────┘
                           │ CGO FFI (JSON)
                    ┌──────▼──────┐
                    │  Rust code  │◄── Aeneas ──► Lean4 proofs
                    │ (verified)  │              (formal theorems)
                    └─────────────┘
```

The core `user_meets_requirements` function is implemented in Rust using a
subset compatible with [Aeneas](https://github.com/AeneasVerif/aeneas), which
auto-translates the Rust code to Lean4 for formal theorem proving.

The Go code calls the Rust implementation via CGO/FFI, and test suites verify
that the Rust implementation produces identical results to the existing Go
implementation (`UserMeetsRequirements` in `lib/accesslists/hierarchy.go`).

## What is verified

The `UserMeetsRequirements` function determines if a user satisfies an access
list's membership requirements. A user meets the requirements if and only if:

1. The requirements are empty (no roles or traits required), OR
2. The user has ALL required roles AND ALL required trait key-value pairs.

### Proven theorems (in `lean4/FormalVerification/Theorems.lean`)

1. **Empty requirements always pass** - any user passes empty requirements
2. **Missing role causes failure** - a user missing any required role fails
3. **Roles-only requirements** - if a user has all required roles (no traits), they pass
4. **Determinism** - same inputs always produce the same output
5. **Monotonicity** - adding a role to a user can only help (not proven fully, lemma provided)

## Prerequisites

### Required (for Rust + Go)
- Rust toolchain (stable)
- Go 1.22+

### Required (for Lean4 proofs)
- [elan](https://github.com/leanprover/elan) (Lean4 version manager)
  ```bash
  curl -sSf https://raw.githubusercontent.com/leanprover/elan/master/elan-init.sh | sh
  ```

### Required (for Aeneas translation)
- [Aeneas](https://github.com/AeneasVerif/aeneas) and charon
- OCaml 5.3.0+ with opam

  ```bash
  # Install OCaml dependencies
  opam switch create 5.3.0
  opam install ppx_deriving visitors easy_logging zarith yojson core_unix \
    odoc ocamlgraph menhir ocamlformat.0.27.0 unionFind zarith progress domainslib

  # Clone and build Aeneas (includes charon)
  git clone https://github.com/AeneasVerif/aeneas.git
  cd aeneas
  make setup-charon
  make
  export PATH="$PWD/bin:$PATH"
  ```

## Quick start

### Build and test Rust + Go

```bash
# Run Rust unit tests
make rust-test

# Build Rust with target triple (needed for CGO linking)
make rust-build-target

# Run Go equivalence tests (compares Go vs Rust FFI results)
make go-test
```

### Verify Lean4 proofs

```bash
# Check all proofs (requires elan/lake)
make lean-verify
```

### Full pipeline (Aeneas translation)

```bash
# Translate Rust to Lean4 (requires charon + aeneas)
make aeneas-translate

# Then verify proofs
make lean-verify
```

## Pinned versions

| Tool | Version |
|------|---------|
| Lean4 | v4.28.0-rc1 |
| Aeneas | build-2026.04.10 (commit 864eddb) |
| Charon | 419f53b |

## Directory structure

```
verified/
  rust/                       Rust implementation
    src/lib.rs                  Core verified logic (Aeneas-compatible)
    src/ffi.rs                  C FFI exports (JSON-based)
    build.rs                    cbindgen header generation
    Cargo.toml
  lean4/                      Lean4 formal verification
    FormalVerification/
      Theorems.lean             Hand-written theorems and proofs
    lakefile.toml               Lake build configuration
    lean-toolchain              Pinned Lean4 version
  scripts/
    aeneas-translate.sh         Rust → Lean4 translation pipeline
    verify.sh                   Lean4 proof checking
  verified.go                 Go FFI wrapper (build tag: verified_accesslists)
  verified_nop.go             Stub for builds without Rust
  verified_test.go            Go ↔ Rust equivalence tests
  Makefile
  README.md
```
