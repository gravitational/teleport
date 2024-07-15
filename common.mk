# Common makefile shared between Teleport OSS and Ent.

# libbpf version required by the build.
LIBBPF_VER := 1.2.2

# Is this build targeting the same OS & architecture it is being compiled on, or
# will it require cross-compilation? We need to know this (especially for ARM) so we
# can set the cross-compiler path (and possibly feature flags) correctly.
IS_NATIVE_BUILD ?= $(if $(filter $(ARCH), $(shell go env GOARCH)),"yes","no")

# BPF support will only be built into Teleport if headers exist at build time.
BPF_MESSAGE := without-BPF-support

# We don't compile BPF for anything except linux/amd64 and linux/arm64 for now,
# as other builds have compilation issues that require fixing.
with_bpf := no
ifeq ("$(OS)","linux")
# True if $ARCH == amd64 || $ARCH == arm64
ifneq (,$(filter "$(ARCH)","amd64" "arm64"))
# We only support BPF in native builds
ifeq ($(IS_NATIVE_BUILD),"yes")
ifneq ("$(wildcard /usr/libbpf-${LIBBPF_VER}/include/bpf/bpf.h)","")
with_bpf := yes
BPF_TAG := bpf
BPF_MESSAGE := with-BPF-support
CLANG ?= $(shell which clang || which clang-12)
LLVM_STRIP ?= $(shell which llvm-strip || which llvm-strip-12)
KERNEL_ARCH := $(shell uname -m | sed 's/x86_64/x86/g; s/aarch64/arm64/g')
INCLUDES :=
ER_BPF_BUILDDIR := lib/bpf/bytecode

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

# Include libbpf dependency. We don't use pkg-config here because we use specific version of libbpf
# and we don't want to include the system libbpf.
STATIC_LIBS += -L/usr/libbpf-${LIBBPF_VER}/lib64/ -lbpf

# Check if pkgconf is installed
PKGCONF := $(shell which pkgconf 2>/dev/null)
ifeq ($(PKGCONF),)
    PKGCONF := $(shell which pkg-config 2>/dev/null)
endif

# Check if libelf is available
HAVE_LIBELF := $(shell $(PKGCONF) --exists libelf && echo 1 || echo 0)

# If pkgconf and libelf are available, use them to get the static libraries
# and all required dependencies for the Teleport build.
# If not, use the default libraries (libelf, libz) and hope for the best.
# This fallback used to work until Ubuntu 24.04 which compiles with libzstd by default.
ifeq ($(HAVE_LIBELF),1)
    STATIC_LIBS += $(shell $(PKGCONF) --static --libs libelf)
else
    STATIC_LIBS += -lelf -lz
endif

# Link static version of libraries required by Teleport (bpf, pcsc) to reduce system dependencies. Avoid dependencies on dynamic libraries if we already link the static version using --as-needed.
CGOFLAG = CGO_ENABLED=1 CGO_CFLAGS="-I/usr/libbpf-${LIBBPF_VER}/include" CGO_LDFLAGS="-Wl,-Bstatic $(STATIC_LIBS) -Wl,-Bdynamic -Wl,--as-needed"
CGOFLAG_TSH = CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-Bstatic $(STATIC_LIBS_TSH) -Wl,-Bdynamic -Wl,--as-needed"
endif # bpf/bpf.h found
endif # IS_NATIVE_BUILD == yes
endif # ARCH == amd64 OR arm64
endif # OS == linux
