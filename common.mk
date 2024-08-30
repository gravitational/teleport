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
# libbpf detection
#
# Requirements for building with BPF support:
# * clang and llvm-strip programs
# * linux on amd64 or amd64
# * libbpf 1.2.2 (either in /usr/libbpf-1.2.2 or pkg-config)
#   * The centos7 buildbox puts this in /usr/libbpf-1.2.2
#   * The ng buildbox has a pkg-config file for it
#   * Native/local builds have a pkg-config file for it
# * Either using a cross-compiling buildbox or is a native build
#
# The default is without BPF support unless all the critera are met.
#
# TODO(camh): Remove /usr/libbpf-1.2.2 when old buildboxes are replaced by ng

with_bpf := no
BPF_MESSAGE := without-BPF-support

CLANG ?= $(shell which clang || which clang-12)
LLVM_STRIP ?= $(shell which llvm-strip || which llvm-strip-12)

# libbpf version required by the build.
LIBBPF_VER := 1.2.2

FOUND_LIBBPF :=

ifneq (,$(wildcard /usr/libbpf-$(LIBBPF_VER)))
FOUND_LIBBPF := true
LIBBPF_INCLUDES := -I/usr/libbpf-$(LIBBPF_VER)/include
LIBBPF_LIBS := -L/usr/libbpf-$(LIBBPF_VER)/lib64 -lbpf
# libbpf needs libelf. Try to find it with pkg-config/pkgconf and fallback to
# hard-coded defaults if pkg-config says nothing.
LIBBPF_LIBS += $(or $(shell $(PKGCONF) --silence-errors --static --libs libelf),-lelf -lz)
else ifneq (,$(shell $(PKGCONF) --exists 'libbpf = $(LIBBPF_VER)' && echo true))
FOUND_LIBBPF := true
LIBBPF_INCLUDES := $(shell $(PKGCONF) --cflags libbpf)
LIBBPF_LIBS := $(shell $(PKGCONF) --libs --static libbpf)
endif

# Is this build targeting the same OS & architecture it is being compiled on, or
# will it require cross-compilation? We need to know this (especially for ARM) so we
# can set the cross-compiler path (and possibly feature flags) correctly.
IS_NATIVE_BUILD ?= $(filter $(ARCH),$(shell go env GOARCH))
IS_CROSS_COMPILE_BB = $(filter $(BUILDBOX_MODE),cross)

# Only build with BPF if clang and llvm-strip are installed.
ifneq (,$(and $(CLANG),$(LLVM_STRIP)))

# Only build with BPF for linux/amd64 and linux/arm64.
# Other builds have compilation issues that require fixing.
ifneq (,$(filter $(OS)/$(ARCH),linux/amd64 linux/arm64))

# Only build with BPF if we found the right version installed
ifneq (,$(FOUND_LIBBPF))

# Only build with BPF if its a native build or in a cross-compiling buildbox.
ifneq (,$(or $(IS_NATIVE_BUILD),$(IS_CROSS_COMPILE_BB)))

with_bpf := yes
BPF_TAG := bpf
BPF_MESSAGE := with-BPF-support
KERNEL_ARCH := $(shell uname -m | sed 's/x86_64/x86/g; s/aarch64/arm64/g')
ER_BPF_BUILDDIR := lib/bpf/bytecode
BPF_INCLUDES := $(LIBBPF_INCLUDES)
STATIC_LIBS += $(LIBBPF_LIBS)

# Get Clang's default includes on this system. We'll explicitly add these dirs
# to the includes list when compiling with `-target bpf` because otherwise some
# architecture-specific dirs will be "missing" on some architectures/distros -
# headers such as asm/types.h, asm/byteorder.h, asm/socket.h, asm/sockios.h,
# sys/cdefs.h etc. might be missing.
#
# Use '-idirafter': Don't interfere with include mechanics except where the
# build would have failed anyways.
CLANG_BPF_SYS_INCLUDES = $(shell $(CLANG) -v -E - </dev/null 2>&1 \
	| sed -n '/<...> search starts here:/,/End of search list./{ s| \(/.*\)|-idirafter \1|p }')

# Link static version of libraries required by Teleport (bpf, pcsc) to reduce
# system dependencies. Avoid dependencies on dynamic libraries if we already
# link the static version using --as-needed.
CGOFLAG = CGO_ENABLED=1 CGO_CFLAGS="$(BPF_INCLUDES)" CGO_LDFLAGS="-Wl,-Bstatic $(STATIC_LIBS) -Wl,-Bdynamic -Wl,--as-needed"
CGOFLAG_TSH = CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-Bstatic $(STATIC_LIBS_TSH) -Wl,-Bdynamic -Wl,--as-needed"

endif # IS_NATIVE_BUILD || IS_CROSS_COMPILE_BB
endif # libbpf found
endif # OS/ARCH == linux/amd64 OR linux/arm64
endif # clang and llvm-strip found

.PHONY: diag-bpf-vars
diag-bpf-vars:
	@echo clang: $(CLANG)
	@echo llvm-strip: $(LLVM_STRIP)
	@echo os/arch: $(OS)-$(ARCH)
	@echo found bpf: $(FOUND_LIBBPF)
	@echo is-native: $(IS_NATIVE_BUILD)
	@echo is-cross: $(IS_CROSS_COMPILE_BB)
	@echo buildbox-mode: $(BUILDBOX_MODE)
