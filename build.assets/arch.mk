# These variables are extracted from build.assets/Makefile so they can be imported
# by other Makefiles

# Eval $(ARCH) one time max, or not at all if $(ARCH) is not used.
# Calling `go` can emit an error if not installed, which we do not
# want to do if $(ARCH) is never used.
# https://make.mad-scientist.net/deferred-simple-variable-expansion/
ARCH = $(eval ARCH := $$(shell GOTOOLCHAIN=local go env GOARCH))$(ARCH)

HOST_ARCH := $(shell uname -m)

RUNTIME_ARCH_x86_64 := amd64
# uname returns different value on Linux (aarch64) and macOS (arm64).
RUNTIME_ARCH_arm64 := arm64
RUNTIME_ARCH_aarch64 := arm64
RUNTIME_ARCH := $(RUNTIME_ARCH_$(HOST_ARCH))
