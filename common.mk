# Common makefile shared between Teleport OSS and Ent.

# -----------------------------------------------------------------------------
# pkg-config / pkgconf
# Set $(PKGCONF) to either pkgconf or pkg-config if either are installed
# or /usr/bin/false if not. When it is set to "false", running $(PKGCONF)
# will exit non-zero with no output.
#
# Before GNU make 4.4, exported variables were not exported for $(shell ...)
# expressions, so explicitly set PKG_CONFIG_PATH when running $(PKGCONF).

PKGCONF := PKG_CONFIG_PATH=$(PKG_CONFIG_PATH) $(firstword $(shell which pkgconf pkg-config false 2>/dev/null))

# -----------------------------------------------------------------------------
# Requirements for building with BPF support:
# * linux on amd64 or amd64
# * Either using a cross-compiling buildbox or is a native build
#
# The default is without BPF support unless all the critera are met.

with_bpf := no
BPF_MESSAGE := without-BPF-support

# Is this build targeting the same OS & architecture it is being compiled on, or
# will it require cross-compilation? We need to know this (especially for ARM) so we
# can set the cross-compiler path (and possibly feature flags) correctly.
IS_NATIVE_BUILD ?= $(filter $(ARCH),$(shell go env GOARCH))
IS_CROSS_COMPILE_BB = $(filter $(BUILDBOX_MODE),cross)

# Only build with BPF for linux/amd64 and linux/arm64.
# Other builds have compilation issues that require fixing.
ifneq (,$(filter $(OS)/$(ARCH),linux/amd64 linux/arm64))

# Only build with BPF if its a native build or in a cross-compiling buildbox.
ifneq (,$(or $(IS_NATIVE_BUILD),$(IS_CROSS_COMPILE_BB)))

with_bpf := yes
BPF_TAG := bpf
BPF_MESSAGE := with-BPF-support
INCLUDES :=

# Link static version of libraries required by Teleport (pcsc) to reduce system dependencies. Avoid dependencies on dynamic libraries if we already link the static version using --as-needed.
CGOFLAG = CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-Bstatic $(STATIC_LIBS) -Wl,-Bdynamic -Wl,--as-needed"
CGOFLAG_TSH = CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-Bstatic $(STATIC_LIBS_TSH) -Wl,-Bdynamic -Wl,--as-needed"

endif # IS_NATIVE_BUILD || IS_CROSS_COMPILE_BB
endif # OS/ARCH == linux/amd64 OR linux/arm64

.PHONY: diag-bpf-vars
diag-bpf-vars:
	@echo os/arch: $(OS)-$(ARCH)
	@echo is-native: $(IS_NATIVE_BUILD)
	@echo is-cross: $(IS_CROSS_COMPILE_BB)
	@echo buildbox-mode: $(BUILDBOX_MODE)

# Dir of last included file, in this case common.mk:
COMMON_MK_DIR := $(dir $(lastword $(MAKEFILE_LIST)))
# This allows $(GOTESTSUM) in any Makefile that includes common.mk:
TOOLS_DIR := $(abspath $(COMMON_MK_DIR)/build.assets/tools)
GOTESTSUM = "$$( GOWORK=off go -C $(TOOLS_DIR)/gotestsum tool -n gotestsum )"
GCI = "$$( GOWORK=off go -C $(TOOLS_DIR)/gci tool -n gci )"
GODA = "$$( GOWORK=off go -C $(TOOLS_DIR)/goda tool -n goda )"
