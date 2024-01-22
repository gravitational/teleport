# These variables are extracted from build.assets/Makefile so they can be imported
# by other Makefiles
ifeq ($(origin ARCH), undefined) # avoid use of "ARCH ?= $(...)" as the lazy loading will repeatedly re-run the same shell command
GO_BINARY := $(shell command -v go 2>/dev/null)
ifndef GO_BINARY
$(error go not installed to determine ARCH))
endif
ARCH := $(shell go env GOARCH)
endif

HOST_ARCH := $(shell uname -m)
RUNTIME_ARCH_x86_64 := amd64
# uname returns different value on Linux (aarch64) and macOS (arm64).
RUNTIME_ARCH_arm64 := arm64
RUNTIME_ARCH_aarch64 := arm64
RUNTIME_ARCH := $(RUNTIME_ARCH_$(HOST_ARCH))
