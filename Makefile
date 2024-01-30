VERSION=14.0.0-dev

DOCKER_IMAGE ?= teleport

GOPATH ?= $(shell go env GOPATH)
# This directory will be the real path of the directory of the first Makefile in the list.
MAKE_DIR := $(dir $(realpath $(firstword $(MAKEFILE_LIST))))

# If set to 1, webassets are not built.
WEBASSETS_SKIP_BUILD ?= 0

# These are standard autotools variables, don't change them please
ifneq ("$(wildcard /bin/bash)","")
SHELL := /bin/bash -o pipefail
endif
BUILDDIR ?= build
BINDIR ?= /usr/local/bin
DATADIR ?= /usr/local/share/teleport
ADDFLAGS ?=
PWD ?= `pwd`
GIT ?= git
TELEPORT_DEBUG ?= false
GITTAG=v$(VERSION)
CGOFLAG ?= CGO_ENABLED=1

# RELEASE_DIR is where the release artifacts (tarballs, pacakges, etc) are put. It
# should be an absolute directory as it is used by e/Makefile too, from the e/ directory.
RELEASE_DIR := $(CURDIR)/$(BUILDDIR)/artifacts

# When TELEPORT_DEBUG is true, set flags to produce
# debugger-friendly builds.
ifeq ("$(TELEPORT_DEBUG)","true")
BUILDFLAGS ?= $(ADDFLAGS) -gcflags=all="-N -l"
else
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s' -trimpath
endif


GO_ENV_OS := $(shell go env GOOS)
OS ?= $(GO_ENV_OS)

GO_ENV_ARCH := $(shell go env GOARCH)
ARCH ?= $(GO_ENV_ARCH)

FIPS ?=
RELEASE = teleport-$(GITTAG)-$(OS)-$(ARCH)-bin


# Include common makefile shared between OSS and Ent.
include common.mk


BINS_default = teleport tctl tsh tbot
BINS_windows = tsh
BINS = $(or $(BINS_$(OS)),$(BINS_default))
BINARIES = $(addprefix $(BUILDDIR)/,$(BINS))

# FIPS support must be requested at build time.
FIPS_MESSAGE := without-FIPS-support
ifneq ("$(FIPS)","")
FIPS_TAG := fips
FIPS_MESSAGE := with-FIPS-support
RELEASE = teleport-$(GITTAG)-$(OS)-$(ARCH)-fips-bin
endif

# PAM support will only be built into Teleport if headers exist at build time.
PAM_MESSAGE := without-PAM-support
ifneq ("$(wildcard /usr/include/security/pam_appl.h)","")
PAM_TAG := pam
PAM_MESSAGE := with-PAM-support
else
# PAM headers for Darwin live under /usr/local/include/security instead, as SIP
# prevents us from modifying/creating /usr/include/security on newer versions of MacOS
ifneq ("$(wildcard /usr/local/include/security/pam_appl.h)","")
PAM_TAG := pam
PAM_MESSAGE := with-PAM-support
endif
endif


# darwin universal (Intel + Apple Silicon combined) binary support
RELEASE_darwin_arm64 = $(RELEASE_DIR)/teleport-$(GITTAG)-darwin-arm64-bin.tar.gz
RELEASE_darwin_amd64 = $(RELEASE_DIR)/teleport-$(GITTAG)-darwin-amd64-bin.tar.gz
BUILDDIR_arm64 = $(BUILDDIR)/arm64
BUILDDIR_amd64 = $(BUILDDIR)/amd64
# TARBINS is the path of the binaries in the release tarballs
TARBINS = $(addprefix teleport/,$(BINS))

# Check if rust and cargo are installed before compiling
CHECK_CARGO := $(shell cargo --version 2>/dev/null)
CHECK_RUST := $(shell rustc --version 2>/dev/null)

RUST_TARGET_ARCH ?= $(CARGO_TARGET_$(OS)_$(ARCH))

CARGO_TARGET_darwin_amd64 := x86_64-apple-darwin
CARGO_TARGET_darwin_arm64 := aarch64-apple-darwin
CARGO_TARGET_linux_arm64 := aarch64-unknown-linux-gnu
CARGO_TARGET_linux_amd64 := x86_64-unknown-linux-gnu

CARGO_TARGET := --target=${CARGO_TARGET_${OS}_${ARCH}}

# If set to 1, Windows RDP client is not built.
RDPCLIENT_SKIP_BUILD ?= 0

# Enable Windows RDP client build?
with_rdpclient := no
RDPCLIENT_MESSAGE := without-Windows-RDP-client

ifeq ($(RDPCLIENT_SKIP_BUILD),0)
ifneq ($(CHECK_RUST),)
ifneq ($(CHECK_CARGO),)

# Do not build RDP client on ARM or 386.
ifneq ("$(ARCH)","arm")
ifneq ("$(ARCH)","386")
# with_rdpclient := yes
with_rdpclient := no
RDPCLIENT_MESSAGE := with-Windows-RDP-client
RDPCLIENT_TAG := desktop_access_rdp
endif
endif

endif
endif
endif

# Set C_ARCH for building libfido2 and dependencies. ARCH is the Go
# architecture which uses different names for architectures than C
# uses. Export it for the build.assets/build-fido2-macos.sh script.
C_ARCH_amd64 = x86_64
C_ARCH = $(or $(C_ARCH_$(ARCH)),$(ARCH))
export C_ARCH

# Enable libfido2 for testing?
# Eagerly enable if we detect the package, we want to test as much as possible.
ifeq ("$(shell pkg-config libfido2 2>/dev/null; echo $$?)", "0")
LIBFIDO2_TEST_TAG := libfido2
endif

# Build tsh against libfido2?
# FIDO2=yes and FIDO2=static enable static libfido2 builds.
# FIDO2=dynamic enables dynamic libfido2 builds.
LIBFIDO2_MESSAGE := without-libfido2
ifneq (, $(filter $(FIDO2), yes static))
LIBFIDO2_MESSAGE := with-libfido2
LIBFIDO2_BUILD_TAG := libfido2 libfido2static
else ifeq ("$(FIDO2)", "dynamic")
LIBFIDO2_MESSAGE := with-libfido2
LIBFIDO2_BUILD_TAG := libfido2
endif

# Enable Touch ID builds?
# Only build if TOUCHID=yes to avoid issues when cross-compiling to 'darwin'
# from other systems.
TOUCHID_MESSAGE := without-Touch-ID
ifeq ("$(TOUCHID)", "yes")
TOUCHID_MESSAGE := with-Touch-ID
TOUCHID_TAG := touchid
endif

# Enable PIV for testing?
# Eagerly enable if we detect the dynamic libpcsclite library, we want to test as much as possible.
ifeq ("$(shell pkg-config libpcsclite 2>/dev/null; echo $$?)", "0")
# This test tag should not be used for builds/releases, only tests.
PIV_TEST_TAG := piv
endif

# Build teleport/api with PIV? This requires the libpcsclite library for linux.
#
# PIV=yes and PIV=static enable static piv builds. This is used by the build
# process to link a static library of libpcsclite for piv-go to connect to.
#
# PIV=dynamic enables dynamic piv builds. This can be used for local
# builds and runs utilizing a dynamic libpcsclite library - `apt get install libpcsclite-dev`
PIV_MESSAGE := without-PIV-support
ifneq (, $(filter $(PIV), yes static dynamic))
PIV_MESSAGE := with-PIV-support
PIV_BUILD_TAG := piv
ifneq ("$(PIV)", "dynamic")
# Link static pcsc libary. By default, piv-go will look for the dynamic library.
# https://github.com/go-piv/piv-go/blob/master/piv/pcsc_unix.go#L23
STATIC_LIBS += -lpcsclite
STATIC_LIBS_TSH += -lpcsclite
endif
endif

# Reproducible builds are only available on select targets, and only when OS=linux.
REPRODUCIBLE ?=
ifneq ("$(OS)","linux")
REPRODUCIBLE = no
endif

# On Windows only build tsh. On all other platforms build teleport, tctl,
# and tsh.
BINS_default = teleport tctl tsh tbot
BINS_windows = tsh
BINS = $(or $(BINS_$(OS)),$(BINS_default))
BINARIES = $(addprefix $(BUILDDIR)/,$(BINS))

# Joins elements of the list in arg 2 with the given separator.
#   1. Element separator.
#   2. The list.
EMPTY :=
SPACE := $(EMPTY) $(EMPTY)
join-with = $(subst $(SPACE),$1,$(strip $2))

# Separate TAG messages into comma-separated WITH and WITHOUT lists for readability.

COMMA := ,
MESSAGES := $(PAM_MESSAGE) $(FIPS_MESSAGE) $(BPF_MESSAGE) $(RDPCLIENT_MESSAGE) $(LIBFIDO2_MESSAGE) $(TOUCHID_MESSAGE) $(PIV_MESSAGE)
WITH := $(subst -," ",$(call join-with,$(COMMA) ,$(subst with-,,$(filter with-%,$(MESSAGES)))))
WITHOUT := $(subst -," ",$(call join-with,$(COMMA) ,$(subst without-,,$(filter without-%,$(MESSAGES)))))
RELEASE_MESSAGE := "Building with GOOS=$(OS) GOARCH=$(ARCH) REPRODUCIBLE=$(REPRODUCIBLE) and with $(WITH) and without $(WITHOUT)."

# On platforms that support reproducible builds, ensure the archive is created in a reproducible manner.
TAR_FLAGS ?=
ifeq ("$(REPRODUCIBLE)","yes")
TAR_FLAGS = --sort=name --owner=root:0 --group=root:0 --mtime='UTC 2015-03-02' --format=gnu
endif

VERSRC = version.go gitref.go api/version.go

KUBECONFIG ?=
TEST_KUBE ?=
export

TEST_LOG_DIR = ${abspath ./test-logs}

# Set CGOFLAG and BUILDFLAGS as needed for the OS/ARCH.
ifeq ("$(OS)","linux")
# True if $ARCH == amd64 || $ARCH == arm64
ifeq ("$(ARCH)","arm64")
	ifeq ($(IS_NATIVE_BUILD),"no")
		CGOFLAG += CC=aarch64-linux-gnu-gcc
	endif
else ifeq ("$(ARCH)","arm")
CGOFLAG = CGO_ENABLED=1

# ARM builds need to specify the correct C compiler
ifeq ($(IS_NATIVE_BUILD),"no")
CC=arm-linux-gnueabihf-gcc
endif

# Add -debugtramp=2 to work around 24 bit CALL/JMP instruction offset.
BUILDFLAGS = $(ADDFLAGS) -ldflags '-w -s -debugtramp=2' -trimpath
endif
endif # OS == linux

# Windows requires extra parameters to cross-compile with CGO.
ifeq ("$(OS)","windows")
ARCH ?= amd64
ifneq ("$(ARCH)","amd64")
$(error "Building for windows requires ARCH=amd64")
endif
CGOFLAG = CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++
BUILDFLAGS = $(ADDFLAGS) -ldflags '-w -s' -trimpath -buildmode=exe
endif

CGOFLAG_TSH ?= $(CGOFLAG)

# Map ARCH into the architecture flag for electron-builder if they
# are different to the Go $(ARCH) we use as an input.
ELECTRON_BUILDER_ARCH_amd64 = x64
ELECTRON_BUILDER_ARCH = $(or $(ELECTRON_BUILDER_ARCH_$(ARCH)),$(ARCH))


.PHONY: all
all: version
	@echo "---> Building OSS binaries."
	$(MAKE) $(BINARIES)


# By making these 3 targets below (tsh, tctl and teleport) PHONY we are solving
# several problems:
# * Build will rely on go build internal caching https://golang.org/doc/go1.10 at all times
# * Manual change detection was broken on a large dependency tree
# If you are considering changing this behavior, please consult with dev team first
.PHONY: $(BUILDDIR)/tctl
$(BUILDDIR)/tctl:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "$(PAM_TAG) $(FIPS_TAG) $(PIV_BUILD_TAG)" -o $(BUILDDIR)/tctl $(BUILDFLAGS) ./tool/tctl

.PHONY: $(BUILDDIR)/teleport
$(BUILDDIR)/teleport: ensure-webassets bpf-bytecode rdpclient
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "webassets_embed $(PAM_TAG) $(FIPS_TAG) $(BPF_TAG) $(WEBASSETS_TAG)  $(PIV_BUILD_TAG)" -o $(BUILDDIR)/teleport $(BUILDFLAGS) ./tool/teleport

# NOTE: Any changes to the `tsh` build here must be copied to `windows.go` in Dronegen until
# 		we can use this Makefile for native Windows builds.
.PHONY: $(BUILDDIR)/tsh
$(BUILDDIR)/tsh:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG_TSH) go build -tags "$(FIPS_TAG) $(LIBFIDO2_BUILD_TAG) $(TOUCHID_TAG) $(PIV_BUILD_TAG)" -o $(BUILDDIR)/tsh $(BUILDFLAGS) ./tool/tsh

.PHONY: $(BUILDDIR)/tbot
$(BUILDDIR)/tbot:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "$(FIPS_TAG)" -o $(BUILDDIR)/tbot $(BUILDFLAGS) ./tool/tbot

# This rule triggers re-generation of version files if Makefile changes.
.PHONY: version
version: $(VERSRC)

# This rule triggers re-generation of version files specified if Makefile changes.
$(VERSRC): Makefile
	VERSION=$(VERSION) $(MAKE) -f version.mk setver

.PHONY: ensure-webassets
ensure-webassets:
	@if [[ "${WEBASSETS_SKIP_BUILD}" -eq 1 ]]; then mkdir -p webassets/teleport && mkdir -p webassets/teleport/app && cp web/packages/build/index.ejs webassets/teleport/index.html; \
	else MAKE="$(MAKE)" "$(MAKE_DIR)/build.assets/build-webassets-if-changed.sh" OSS webassets/oss-sha build-ui web; fi

.PHONY: build-ui
build-ui: ensure-js-deps
	@[ "${WEBASSETS_SKIP_BUILD}" -eq 1 ] || yarn build-ui-oss


.PHONY: ensure-js-deps
ensure-js-deps:
	@if [[ "${WEBASSETS_SKIP_BUILD}" -eq 1 ]]; then mkdir -p webassets/teleport && touch webassets/teleport/index.html; \
	else yarn install --ignore-scripts; fi
	
#
# BPF support (IF ENABLED)
# Requires a recent version of clang and libbpf installed.
#
ifeq ("$(with_bpf)","yes")
$(ER_BPF_BUILDDIR):
	mkdir -p $(ER_BPF_BUILDDIR)

$(RS_BPF_BUILDDIR):
	mkdir -p $(RS_BPF_BUILDDIR)

# Build BPF code
$(ER_BPF_BUILDDIR)/%.bpf.o: bpf/enhancedrecording/%.bpf.c $(wildcard bpf/*.h) | $(ER_BPF_BUILDDIR)
	$(CLANG) -g -O2 -target bpf -D__TARGET_ARCH_$(KERNEL_ARCH) -I/usr/libbpf-${LIBBPF_VER}/include $(INCLUDES) $(CLANG_BPF_SYS_INCLUDES) -c $(filter %.c,$^) -o $@
	$(LLVM_STRIP) -g $@ # strip useless DWARF info

# Build BPF code
$(RS_BPF_BUILDDIR)/%.bpf.o: bpf/restrictedsession/%.bpf.c $(wildcard bpf/*.h) | $(RS_BPF_BUILDDIR)
	$(CLANG) -g -O2 -target bpf -D__TARGET_ARCH_$(KERNEL_ARCH) -I/usr/libbpf-${LIBBPF_VER}/include $(INCLUDES) $(CLANG_BPF_SYS_INCLUDES) -c $(filter %.c,$^) -o $@
	$(LLVM_STRIP) -g $@ # strip useless DWARF info

.PHONY: bpf-rs-bytecode
bpf-rs-bytecode: $(RS_BPF_BUILDDIR)/restricted.bpf.o

.PHONY: bpf-er-bytecode
bpf-er-bytecode: $(ER_BPF_BUILDDIR)/command.bpf.o $(ER_BPF_BUILDDIR)/disk.bpf.o $(ER_BPF_BUILDDIR)/network.bpf.o $(ER_BPF_BUILDDIR)/counter_test.bpf.o

.PHONY: bpf-bytecode
bpf-bytecode: bpf-er-bytecode bpf-rs-bytecode

# Generate vmlinux.h based on the installed kernel
.PHONY: update-vmlinux-h
update-vmlinux-h:
	bpftool btf dump file /sys/kernel/btf/vmlinux format c >bpf/vmlinux.h

else
.PHONY: bpf-bytecode
bpf-bytecode:
endif

ifeq ("$(with_rdpclient)", "yes")
.PHONY: rdpclient
rdpclient:
ifneq ("$(FIPS)","")
	cargo build -p rdp-client --features=fips --release $(CARGO_TARGET)
else
	cargo build -p rdp-client --release $(CARGO_TARGET)
endif
else
.PHONY: rdpclient
rdpclient:
endif



.PHONY: docker-ui
docker-ui:
	$(MAKE) -C build.assets ui

# .PHONY: rustup-install-target-toolchain
# rustup-install-target-toolchain: 
# 	RUST_VERSION := $(shell $(MAKE) --no-print-directory -C build.assets print-rust-version)
# rustup-install-target-toolchain:
# 	rustup override set $(RUST_VERSION)
# 	rustup target add $(RUST_TARGET_ARCH)
