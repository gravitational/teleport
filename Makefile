# Make targets:
#
#  all    : builds all binaries in development mode
#  full   : builds all binaries for PRODUCTION use
#  release: prepares a release tarball
#  clean  : removes all build artifacts
#  test   : runs tests

# To update the Teleport version, update VERSION variable:
# Naming convention:
#   Stable releases:   "1.0.0"
#   Pre-releases:      "1.0.0-alpha.1", "1.0.0-beta.2", "1.0.0-rc.3"
#   Master/dev branch: "1.0.0-dev"
VERSION=16.4.16

DOCKER_IMAGE ?= teleport

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
TELEPORT_DEBUG ?= false
GITTAG=v$(VERSION)
CGOFLAG ?= CGO_ENABLED=1
KUSTOMIZE_NO_DYNAMIC_PLUGIN ?= kustomize_disable_go_plugin_support
# RELEASE_DIR is where the release artifacts (tarballs, pacakges, etc) are put. It
# should be an absolute directory as it is used by e/Makefile too, from the e/ directory.
RELEASE_DIR := $(CURDIR)/$(BUILDDIR)/artifacts

GO_LDFLAGS ?= -w -s $(KUBECTL_SETVERSION)

# Appending new conditional settings for community build type
# When TELEPORT_DEBUG is true, set flags to produce
# debugger-friendly builds.
ifeq ("$(TELEPORT_DEBUG)","true")
BUILDFLAGS ?= $(ADDFLAGS) -gcflags=all="-N -l"
BUILDFLAGS_TBOT ?= $(ADDFLAGS) -gcflags=all="-N -l"
else
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '$(GO_LDFLAGS)' -trimpath -buildmode=pie
BUILDFLAGS_TBOT ?= $(ADDFLAGS) -ldflags '$(GO_LDFLAGS)' -trimpath
endif

GO_ENV_OS := $(shell go env GOOS)
OS ?= $(GO_ENV_OS)

GO_ENV_ARCH := $(shell go env GOARCH)
ARCH ?= $(GO_ENV_ARCH)

FIPS ?=
RELEASE = teleport-$(GITTAG)-$(OS)-$(ARCH)-bin

# Include common makefile shared between OSS and Ent.
include common.mk

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
CARGO_TARGET_linux_arm := arm-unknown-linux-gnueabihf
CARGO_TARGET_linux_arm64 := aarch64-unknown-linux-gnu
CARGO_TARGET_linux_386 := i686-unknown-linux-gnu
CARGO_TARGET_linux_amd64 := x86_64-unknown-linux-gnu

CARGO_TARGET := --target=$(RUST_TARGET_ARCH)

# If set to 1, Windows RDP client is not built.
RDPCLIENT_SKIP_BUILD ?= 0

# Enable Windows RDP client build?
with_rdpclient := no
RDPCLIENT_MESSAGE := without-Windows-RDP-client

ifeq ($(RDPCLIENT_SKIP_BUILD),0)
ifneq ($(CHECK_RUST),)
ifneq ($(CHECK_CARGO),)

is_fips_on_arm64 := no
ifneq ("$(FIPS)","")
ifeq ("$(ARCH)","arm64")
is_fips_on_arm64 := yes
endif
endif

# Do not build RDP client on 32-bit ARM or 386, or for FIPS builds on arm64.
ifneq ("$(ARCH)","arm")
ifneq ("$(ARCH)","386")
ifneq ("$(is_fips_on_arm64)","yes")
with_rdpclient := yes
RDPCLIENT_MESSAGE := with-Windows-RDP-client
RDPCLIENT_TAG := desktop_access_rdp
endif
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
ifeq ($(FIDO2),)
FIDO2 ?= dynamic
endif
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

# Enable VNet daemon?
# With VNETDAEMON=yes, tsh uses a Launch Daemon to start VNet.
# This requires a signed and bundled tsh.
VNETDAEMON_MESSAGE := without-VNet-daemon
ifeq ("$(VNETDAEMON)", "yes")
VNETDAEMON_MESSAGE := with-VNet-daemon
VNETDAEMON_TAG := vnetdaemon
endif

# Enable PIV test packages for testing.
# This test tag should never be used for builds/releases, only tests.
PIV_TEST_TAG := pivtest

# enable PIV package for linting.
PIV_LINT_TAG := piv

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
BINS_default = teleport tctl tsh tbot fdpass-teleport
BINS_windows = tsh tctl
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
MESSAGES := $(PAM_MESSAGE) $(FIPS_MESSAGE) $(BPF_MESSAGE) $(RDPCLIENT_MESSAGE) $(LIBFIDO2_MESSAGE) $(TOUCHID_MESSAGE) $(PIV_MESSAGE) $(VNETDAEMON_MESSAGE)
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
# Add "-extldflags -Wl,--long-plt" to avoid ld assertion failure on large binaries
GO_LDFLAGS += -extldflags=-Wl,--long-plt -debugtramp=2
endif
endif # OS == linux

ifeq ("$(OS)-$(ARCH)","darwin-arm64")
# Temporary link flags due to changes in Apple's linker
# https://github.com/golang/go/issues/67854
GO_LDFLAGS += -extldflags=-ld_classic
endif

# Windows requires extra parameters to cross-compile with CGO.
ifeq ("$(OS)","windows")
ARCH ?= amd64
ifneq ("$(ARCH)","amd64")
$(error "Building for windows requires ARCH=amd64")
endif
CGOFLAG = CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++
BUILDFLAGS = $(ADDFLAGS) -ldflags '-w -s $(KUBECTL_SETVERSION)' -trimpath -buildmode=pie
BUILDFLAGS_TBOT = $(ADDFLAGS) -ldflags '-w -s $(KUBECTL_SETVERSION)' -trimpath
endif

ifeq ("$(OS)","darwin")
# Set the minimum version for macOS builds for Go, Rust and Xcode builds.
# Note the minimum version for Apple silicon (ARM64) is 11.0 and will be automatically
# clamped to the value for builds of that architecture
MINIMUM_SUPPORTED_MACOS_VERSION = 10.15
MACOSX_VERSION_MIN_FLAG = -mmacosx-version-min=$(MINIMUM_SUPPORTED_MACOS_VERSION)

# Go
CGOFLAG = CGO_ENABLED=1 CGO_CFLAGS=$(MACOSX_VERSION_MIN_FLAG)

# Xcode and rust and Go linking
MACOSX_DEPLOYMENT_TARGET = $(MINIMUM_SUPPORTED_MACOS_VERSION)
export MACOSX_DEPLOYMENT_TARGET
endif

CGOFLAG_TSH ?= $(CGOFLAG)

# Map ARCH into the architecture flag for electron-builder if they
# are different to the Go $(ARCH) we use as an input.
ELECTRON_BUILDER_ARCH_amd64 = x64
ELECTRON_BUILDER_ARCH = $(or $(ELECTRON_BUILDER_ARCH_$(ARCH)),$(ARCH))

#
# 'make all' builds all 4 executables and places them in the current directory.
#
# NOTE: Works the same as `make`. Left for legacy reasons.
.PHONY: all
all: version
	@echo "---> Building OSS binaries."
	$(MAKE) $(BINARIES)

#
# make binaries builds all binaries defined in the BINARIES environment variable
#
.PHONY: binaries
binaries:
	$(MAKE) $(BINARIES)

# Appending new conditional settings for community build type for tools.
ifeq ("$(GITHUB_REPOSITORY_OWNER)","gravitational")
# TELEPORT_LDFLAGS and TOOLS_LDFLAGS if appended will overwrite the previous LDFLAGS set in the BUILDFLAGS.
# This is done here to prevent any changes to the (BUI)LDFLAGS passed to the other binaries
TELEPORT_LDFLAGS ?= -ldflags '$(GO_LDFLAGS) -X github.com/gravitational/teleport/lib/modules.teleportBuildType=community'
TOOLS_LDFLAGS ?= -ldflags '$(GO_LDFLAGS) -X github.com/gravitational/teleport/lib/modules.teleportBuildType=community'
endif

# By making these 3 targets below (tsh, tctl and teleport) PHONY we are solving
# several problems:
# * Build will rely on go build internal caching https://golang.org/doc/go1.10 at all times
# * Manual change detection was broken on a large dependency tree
# If you are considering changing this behavior, please consult with dev team first
#
# NOTE: Any changes to the `tctl` build here must be copied to `build.assets/windows/build.ps1`
# until we can use this Makefile for native Windows builds.
.PHONY: $(BUILDDIR)/tctl
$(BUILDDIR)/tctl:
	@if [[ -z "$(LIBFIDO2_BUILD_TAG)" ]]; then \
		echo 'Warning: Building tctl without libfido2. Install libfido2 to have access to MFA.' >&2; \
	fi
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "$(PAM_TAG) $(FIPS_TAG) $(LIBFIDO2_BUILD_TAG) $(PIV_BUILD_TAG) $(KUSTOMIZE_NO_DYNAMIC_PLUGIN)" -o $(BUILDDIR)/tctl $(BUILDFLAGS) $(TOOLS_LDFLAGS) ./tool/tctl

.PHONY: $(BUILDDIR)/teleport
$(BUILDDIR)/teleport: ensure-webassets bpf-bytecode rdpclient
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "webassets_embed $(PAM_TAG) $(FIPS_TAG) $(BPF_TAG) $(WEBASSETS_TAG) $(RDPCLIENT_TAG) $(PIV_BUILD_TAG) $(KUSTOMIZE_NO_DYNAMIC_PLUGIN)" -o $(BUILDDIR)/teleport $(BUILDFLAGS) $(TELEPORT_LDFLAGS) ./tool/teleport

# NOTE: Any changes to the `tsh` build here must be copied to `build.assets/windows/build.ps1`
# until we can use this Makefile for native Windows builds.
.PHONY: $(BUILDDIR)/tsh
$(BUILDDIR)/tsh: KUBECTL_VERSION ?= $(shell go run ./build.assets/kubectl-version/main.go)
$(BUILDDIR)/tsh: KUBECTL_SETVERSION ?= -X k8s.io/component-base/version.gitVersion=$(KUBECTL_VERSION)
$(BUILDDIR)/tsh:
	@if [[ -z "$(LIBFIDO2_BUILD_TAG)" ]]; then \
		echo 'Warning: Building tsh without libfido2. Install libfido2 to have access to MFA.' >&2; \
	fi
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG_TSH) go build -tags "$(FIPS_TAG) $(LIBFIDO2_BUILD_TAG) $(TOUCHID_TAG) $(PIV_BUILD_TAG) $(VNETDAEMON_TAG) $(KUSTOMIZE_NO_DYNAMIC_PLUGIN)" -o $(BUILDDIR)/tsh $(BUILDFLAGS) $(TOOLS_LDFLAGS) ./tool/tsh

.PHONY: $(BUILDDIR)/tbot
# tbot is CGO-less by default except on Windows because lib/client/terminal/ wants CGO on this OS
$(BUILDDIR)/tbot: TBOT_CGO_FLAGS ?= $(if $(filter windows,$(OS)),$(CGOFLAG))
# Build mode pie requires CGO
$(BUILDDIR)/tbot: BUILDFLAGS_TBOT += $(if $(TBOT_CGO_FLAGS), -buildmode=pie)
$(BUILDDIR)/tbot:
	GOOS=$(OS) GOARCH=$(ARCH) $(TBOT_CGO_FLAGS) go build -tags "$(FIPS_TAG) $(KUSTOMIZE_NO_DYNAMIC_PLUGIN)" -o $(BUILDDIR)/tbot $(BUILDFLAGS_TBOT) $(TOOLS_LDFLAGS) ./tool/tbot

TELEPORT_ARGS ?= start
.PHONY: teleport-hot-reload
teleport-hot-reload:
	CompileDaemon \
		--graceful-kill=true \
		--exclude-dir=".git" \
		--exclude-dir="build" \
		--exclude-dir="e/build" \
		--exclude-dir="e/web/*/node_modules" \
		--exclude-dir="node_modules" \
		--exclude-dir="target" \
		--exclude-dir="web/packages/*/node_modules" \
		--color \
		--log-prefix=false \
		--build="make $(BUILDDIR)/teleport" \
		--command="$(BUILDDIR)/teleport $(TELEPORT_ARGS)"

.PHONY: $(BUILDDIR)/fdpass-teleport
$(BUILDDIR)/fdpass-teleport:
	cd tool/fdpass-teleport && cargo build --release --locked $(CARGO_TARGET)
	install tool/fdpass-teleport/target/$(RUST_TARGET_ARCH)/release/fdpass-teleport $(BUILDDIR)/

#
# BPF support (IF ENABLED)
# Requires a recent version of clang and libbpf installed.
#
ifeq ("$(with_bpf)","yes")
$(ER_BPF_BUILDDIR):
	mkdir -p $(ER_BPF_BUILDDIR)

# Build BPF code
$(ER_BPF_BUILDDIR)/%.bpf.o: bpf/enhancedrecording/%.bpf.c $(wildcard bpf/*.h) | $(ER_BPF_BUILDDIR)
	$(CLANG) -g -O2 -target bpf -D__TARGET_ARCH_$(KERNEL_ARCH) -I/usr/libbpf-${LIBBPF_VER}/include $(INCLUDES) $(CLANG_BPF_SYS_INCLUDES) -c $(filter %.c,$^) -o $@
	$(LLVM_STRIP) -g $@ # strip useless DWARF info

.PHONY: bpf-er-bytecode
bpf-er-bytecode: $(ER_BPF_BUILDDIR)/command.bpf.o $(ER_BPF_BUILDDIR)/disk.bpf.o $(ER_BPF_BUILDDIR)/network.bpf.o $(ER_BPF_BUILDDIR)/counter_test.bpf.o

.PHONY: bpf-bytecode
bpf-bytecode: bpf-er-bytecode

# Generate vmlinux.h based on the installed kernel
.PHONY: update-vmlinux-h
update-vmlinux-h:
	bpftool btf dump file /sys/kernel/btf/vmlinux format c >bpf/vmlinux.h

else
.PHONY: bpf-bytecode
bpf-bytecode:
endif

.PHONY: rdpclient
rdpclient:
ifeq ("$(with_rdpclient)", "yes")
	cargo build -p rdp-client $(if $(FIPS),--features=fips) --release --locked $(CARGO_TARGET)
endif

# Build libfido2 and dependencies for MacOS. Uses exported C_ARCH variable defined earlier.
.PHONY: build-fido2
build-fido2:
	./build.assets/build-fido2-macos.sh build

.PHONY: print-fido2-pkg-path
print-fido2-pkg-path:
	@./build.assets/build-fido2-macos.sh pkg_config_path

#
# make full - Builds Teleport binaries with the built-in web assets and
# places them into $(BUILDDIR). On Windows, this target is skipped because
# only tsh and tctl are built.
#
.PHONY:full
full: WEBASSETS_SKIP_BUILD = 0
full: ensure-webassets
ifneq ("$(OS)", "windows")
	export WEBASSETS_SKIP_BUILD=0
	$(MAKE) all
endif

#
# make full-ent - Builds Teleport enterprise binaries
#
.PHONY:full-ent
full-ent: ensure-webassets-e
ifneq ("$(OS)", "windows")
	@if [ -f e/Makefile ]; then $(MAKE) -C e full; fi
endif

#
# make clean - Removes all build artifacts.
#
.PHONY: clean
clean: clean-ui clean-build

.PHONY: clean-build
clean-build:
	@echo "---> Cleaning up OSS build artifacts."
	rm -rf $(BUILDDIR)
# Check if the variable is set to prevent calling remove on the root directory.
ifneq ($(ER_BPF_BUILDDIR),)
	rm -f $(ER_BPF_BUILDDIR)/*.o
endif
	-cargo clean
	-go clean -cache
	rm -f *.gz
	rm -f *.zip
	rm -f gitref.go
	rm -rf build.assets/tooling/bin
	# Clean up wasm-pack build artifacts
	rm -rf web/packages/teleport/src/ironrdp/pkg/

.PHONY: clean-ui
clean-ui:
	rm -rf webassets/*
	rm -rf web/packages/teleterm/build
	find . -type d -name node_modules -prune -exec rm -rf {} \;

# RELEASE_DIR is where release artifact files are put, such as tarballs, packages, etc.
$(RELEASE_DIR):
	mkdir -p $@

#
# make release - Produces a binary release tarball.
#
.PHONY:
export
release:
	@echo "---> OSS $(RELEASE_MESSAGE)"
ifeq ("$(OS)", "windows")
	$(MAKE) --no-print-directory release-windows
else ifeq ("$(OS)", "darwin")
	$(MAKE) --no-print-directory release-darwin
else
	$(MAKE) --no-print-directory release-unix
endif

# These are aliases used to make build commands uniform.
.PHONY: release-amd64
release-amd64:
	$(MAKE) release ARCH=amd64

.PHONY: release-386
release-386:
	$(MAKE) release ARCH=386

.PHONY: release-arm
release-arm:
	$(MAKE) release ARCH=arm

.PHONY: release-arm64
release-arm64:
	$(MAKE) release ARCH=arm64

#
# make build-archive - Packages the results of a build into a release tarball
#
.PHONY: build-archive
build-archive: | $(RELEASE_DIR)
	@echo "---> Creating OSS release archive."
	mkdir teleport
	cp -rf $(BINARIES) \
		examples \
		build.assets/install\
		README.md \
		CHANGELOG.md \
		build.assets/LICENSE-community \
		teleport/
	echo $(GITTAG) > teleport/VERSION
	tar $(TAR_FLAGS) -c teleport | gzip -n > $(RELEASE).tar.gz
	cp $(RELEASE).tar.gz $(RELEASE_DIR)
	# linux-amd64 generates a centos7-compatible archive. Make a copy with the -centos7 label,
	# for the releases page. We should probably drop that at some point.
	$(if $(filter linux-amd64,$(OS)-$(ARCH)), \
		cp $(RELEASE).tar.gz $(RELEASE_DIR)/$(subst amd64,amd64-centos7,$(RELEASE)).tar.gz \
	)
	rm -rf teleport
	@echo "---> Created $(RELEASE).tar.gz."

#
# make release-unix - Produces binary release tarballs for both OSS and
# Enterprise editions, containing teleport, tctl, tbot and tsh.
#
.PHONY: release-unix
release-unix: clean full build-archive
	@if [ -f e/Makefile ]; then $(MAKE) -C e release; fi

# release-unix-preserving-webassets cleans just the build and not the UI
# allowing webassets to be built in a prior step before building the release.
.PHONY: release-unix-preserving-webassets
release-unix-preserving-webassets: clean-build full build-archive
	@if [ -f e/Makefile ]; then $(MAKE) -C e release; fi

include darwin-signing.mk

.PHONY: release-darwin-unsigned
release-darwin-unsigned: RELEASE:=$(RELEASE)-unsigned
release-darwin-unsigned: full build-archive

.PHONY: release-darwin
ifneq ($(ARCH),universal)
release-darwin: release-darwin-unsigned
	$(NOTARIZE_BINARIES)
	$(MAKE) build-archive
	@if [ -f e/Makefile ]; then $(MAKE) -C e release; fi
else

# release-darwin for ARCH == universal does not build binaries, but instead
# combines previously-built binaries. For this, it depends on the ARM64 and
# AMD64 signed tarballs being built into $(RELEASE_DIR). The dependencies
# expressed here will not make that happen as this is typically done on CI
# where these two tarballs are built in separate pipelines, and copied in for
# the universal build.
#
# For local manual runs, create these tarballs with:
#   make ARCH=arm64 release-darwin
#   make ARCH=amd64 release-darwin
# Ensure you have the rust toolchains for these installed by running
#   make ARCH=arm64 rustup-install-target-toolchain
#   make ARCH=amd64 rustup-install-target-toolchain
release-darwin: $(RELEASE_darwin_arm64) $(RELEASE_darwin_amd64)
	mkdir -p $(BUILDDIR_arm64) $(BUILDDIR_amd64)
	tar -C $(BUILDDIR_arm64) -xzf $(RELEASE_darwin_arm64) --strip-components=1 $(TARBINS)
	tar -C $(BUILDDIR_amd64) -xzf $(RELEASE_darwin_amd64) --strip-components=1 $(TARBINS)
	lipo -create -output $(BUILDDIR)/teleport $(BUILDDIR_arm64)/teleport $(BUILDDIR_amd64)/teleport
	lipo -create -output $(BUILDDIR)/tctl $(BUILDDIR_arm64)/tctl $(BUILDDIR_amd64)/tctl
	lipo -create -output $(BUILDDIR)/tsh $(BUILDDIR_arm64)/tsh $(BUILDDIR_amd64)/tsh
	lipo -create -output $(BUILDDIR)/tbot $(BUILDDIR_arm64)/tbot $(BUILDDIR_amd64)/tbot
	lipo -create -output $(BUILDDIR)/fdpass-teleport $(BUILDDIR_arm64)/fdpass-teleport $(BUILDDIR_amd64)/fdpass-teleport
	$(MAKE) ARCH=universal build-archive
	@if [ -f e/Makefile ]; then $(MAKE) -C e release; fi
endif

#
# make release-windows-unsigned - Produces a binary release archive containing tsh and tctl.
#
.PHONY: release-windows-unsigned
release-windows-unsigned: clean all
	@echo "---> Creating OSS release archive."
	mkdir teleport
	cp -rf $(BUILDDIR)/* \
		README.md \
		CHANGELOG.md \
		teleport/
	mv teleport/tsh teleport/tsh-unsigned.exe
	mv teleport/tctl teleport/tctl-unsigned.exe
	echo $(GITTAG) > teleport/VERSION
	zip -9 -y -r -q $(RELEASE)-unsigned.zip teleport/
	rm -rf teleport/
	@echo "---> Created $(RELEASE)-unsigned.zip."

#
# make release-windows - Produces an archive containing a signed release of
# tsh.exe and tctl.exe
#
.PHONY: release-windows
release-windows: release-windows-unsigned
	@if [ ! -f "windows-signing-cert.pfx" ]; then \
		echo "windows-signing-cert.pfx is missing or invalid, cannot create signed archive."; \
		exit 1; \
	fi

	rm -rf teleport
	@echo "---> Extracting $(RELEASE)-unsigned.zip"
	unzip $(RELEASE)-unsigned.zip

	@echo "---> Signing Windows tsh binary."
	@osslsigncode sign \
		-pkcs12 "windows-signing-cert.pfx" \
		-n "Teleport" \
		-i https://goteleport.com \
		-t http://timestamp.digicert.com \
		-h sha2 \
		-in teleport/tsh-unsigned.exe \
		-out teleport/tsh.exe; \
	success=$$?; \
	rm -f teleport/tsh-unsigned.exe; \
	if [ "$${success}" -ne 0 ]; then \
		echo "Failed to sign tsh.exe, aborting."; \
		exit 1; \
	fi

	echo "---> Signing Windows tctl binary."
	@osslsigncode sign \
		-pkcs12 "windows-signing-cert.pfx" \
		-n "Teleport" \
		-i https://goteleport.com \
		-t http://timestamp.digicert.com \
		-h sha2 \
		-in teleport/tctl-unsigned.exe \
		-out teleport/tctl.exe; \
	success=$$?; \
	rm -f teleport/tctl-unsigned.exe; \
	if [ "$${success}" -ne 0 ]; then \
		echo "Failed to sign tctl.exe, aborting."; \
		exit 1; \
	fi

	zip -9 -y -r -q $(RELEASE).zip teleport/
	rm -rf teleport/
	@echo "---> Created $(RELEASE).zip."

#
# make release-connect produces a release package of Teleport Connect.
# It is used only for MacOS releases. Windows releases do not use this
# Makefile. Linux uses the `teleterm` target in build.assets/Makefile.
#
# Either CONNECT_TSH_BIN_PATH or CONNECT_TSH_APP_PATH environment variable
# should be defined for the `pnpm package-term` command to succeed. CI sets
# this appropriately depending on whether a push build is running, or a
# proper release (a proper release needs the APP_PATH as that points to
# the complete signed package). See web/packages/teleterm/README.md for
# details.
.PHONY: release-connect
release-connect: | $(RELEASE_DIR)
	pnpm install --frozen-lockfile
	pnpm build-term
	pnpm package-term -c.extraMetadata.version=$(VERSION) --$(ELECTRON_BUILDER_ARCH)
	# Only copy proper builds with tsh.app to $(RELEASE_DIR)
	# Drop -universal "arch" from dmg name when copying to $(RELEASE_DIR)
	if [ -n "$$CONNECT_TSH_APP_PATH" ]; then \
		TARGET_NAME="Teleport Connect-$(VERSION)-$(ARCH).dmg"; \
		if [ "$(ARCH)" = 'universal' ]; then \
			TARGET_NAME="$${TARGET_NAME/-universal/}"; \
		fi; \
		cp web/packages/teleterm/build/release/"Teleport Connect-$(VERSION)-$(ELECTRON_BUILDER_ARCH).dmg" "$(RELEASE_DIR)/$${TARGET_NAME}"; \
	fi

#
# Remove trailing whitespace in all markdown files under docs/.
#
# Note: this runs in a busybox container to avoid incompatibilities between
# linux and macos CLI tools.
#
.PHONY:docs-fix-whitespace
docs-fix-whitespace:
	docker run --rm -v $(PWD):/teleport busybox \
		find /teleport/docs/ -type f -name '*.md' -exec sed -E -i 's/\s+$$//g' '{}' \;

#
# Test docs for trailing whitespace and broken links
#
.PHONY:docs-test
docs-test: docs-test-whitespace

#
# Check for trailing whitespace in all markdown files under docs/
#
.PHONY:docs-test-whitespace
docs-test-whitespace:
	if find docs/ -type f -name '*.md' | xargs grep -E '\s+$$'; then \
		echo "trailing whitespace found in docs/ (see above)"; \
		echo "run 'make docs-fix-whitespace' to fix it"; \
		exit 1; \
	fi

#
# Builds some tooling for filtering and displaying test progress/output/etc
#
# Deprecated: Use gotestsum instead.
TOOLINGDIR := ${abspath ./build.assets/tooling}
RENDER_TESTS := $(TOOLINGDIR)/bin/render-tests
$(RENDER_TESTS): $(wildcard $(TOOLINGDIR)/cmd/render-tests/*.go)
	cd $(TOOLINGDIR) && go build -o "$@" ./cmd/render-tests

#
# Install gotestsum to parse test output.
#
.PHONY: ensure-gotestsum
ensure-gotestsum:
# Install gotestsum if it's not already installed
 ifeq (, $(shell command -v gotestsum))
	go install gotest.tools/gotestsum@latest
endif

DIFF_TEST := $(TOOLINGDIR)/bin/difftest
$(DIFF_TEST): $(wildcard $(TOOLINGDIR)/cmd/difftest/*.go)
	cd $(TOOLINGDIR) && go build -o "$@" ./cmd/difftest

RERUN := $(TOOLINGDIR)/bin/rerun
$(RERUN): $(wildcard $(TOOLINGDIR)/cmd/rerun/*.go)
	cd $(TOOLINGDIR) && go build -o "$@" ./cmd/rerun

.PHONY: tooling
tooling: ensure-gotestsum $(DIFF_TEST)

#
# Runs all Go/shell tests, called by CI/CD.
#
.PHONY: test
test: test-helm test-sh test-api test-go test-rust test-operator test-terraform-provider

$(TEST_LOG_DIR):
	mkdir $(TEST_LOG_DIR)

.PHONY: helmunit/installed
helmunit/installed:
	@if ! helm unittest -h >/dev/null; then \
		echo 'Helm unittest plugin is required to test Helm charts. Run `helm plugin install https://github.com/quintush/helm-unittest --version 0.2.11` to install it'; \
		exit 1; \
	fi

# The CI environment is responsible for setting HELM_PLUGINS to a directory where
# quintish/helm-unittest is installed.
#
# Github Actions build uses /workspace as homedir and Helm can't pick up plugins by default there,
# so override the plugin location via environemnt variable when running in CI. Github Actions provide CI=true
# environment variable.
.PHONY: test-helm
test-helm: helmunit/installed
	helm unittest -3 --with-subchart=false examples/chart/teleport-cluster
	helm unittest -3 --with-subchart=false examples/chart/teleport-kube-agent
	helm unittest -3 --with-subchart=false examples/chart/teleport-cluster/charts/teleport-operator
	helm unittest -3 --with-subchart=false examples/chart/access/*
	helm unittest -3 --with-subchart=false examples/chart/event-handler
	helm unittest -3 --with-subchart=false examples/chart/tbot

.PHONY: test-helm-update-snapshots
test-helm-update-snapshots: helmunit/installed
	helm unittest -3 -u --with-subchart=false examples/chart/teleport-cluster
	helm unittest -3 -u --with-subchart=false examples/chart/teleport-kube-agent
	helm unittest -3 -u --with-subchart=false examples/chart/teleport-cluster/charts/teleport-operator
	helm unittest -3 -u --with-subchart=false examples/chart/access/*
	helm unittest -3 -u --with-subchart=false examples/chart/event-handler
	helm unittest -3 -u --with-subchart=false examples/chart/tbot

#
# Runs all Go tests except integration, called by CI/CD.
#
.PHONY: test-go
test-go: test-go-prepare test-go-unit test-go-touch-id test-go-vnet-daemon test-go-tsh test-go-chaos

#
# Runs a test to ensure no environment variable leak into build binaries.
# This is typically done as part of the bloat test in CI, but this
# target exists for local testing.
#
.PHONY: test-env-leakage
test-env-leakage:
	$(eval export BUILD_SECRET=FAKE_SECRET)
	$(MAKE) full
	failed=0; \
	for binary in $(BINARIES); do \
		if strings $$binary | grep -q 'FAKE_SECRET'; then \
			echo "Error: $$binary contains FAKE_SECRET"; \
			failed=1; \
		fi; \
	done; \
	if [ $$failed -eq 1 ]; then \
		echo "Environment leak failure"; \
		exit 1; \
	else \
		echo "No environment leak, PASS"; \
	fi

# Runs test prepare steps
.PHONY: test-go-prepare
test-go-prepare: ensure-webassets bpf-bytecode $(TEST_LOG_DIR) ensure-gotestsum $(VERSRC)

# Runs base unit tests
.PHONY: test-go-unit
test-go-unit: FLAGS ?= -race -shuffle on
test-go-unit: SUBJECT ?= $(shell go list ./... | grep -vE 'teleport/(e2e|integration|tool/tsh|integrations/operator|integrations/access|integrations/lib)')
test-go-unit:
	$(CGOFLAG) go test -cover -json -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG) $(LIBFIDO2_TEST_TAG) $(TOUCHID_TAG) $(PIV_TEST_TAG) $(VNETDAEMON_TAG)" $(PACKAGES) $(SUBJECT) $(FLAGS) $(ADDFLAGS) \
		| tee $(TEST_LOG_DIR)/unit.json \
		| gotestsum --raw-command -- cat

# Runs tbot unit tests
.PHONY: test-go-unit-tbot
test-go-unit-tbot: FLAGS ?= -race -shuffle on
test-go-unit-tbot:
	$(CGOFLAG) go test -cover -json $(FLAGS) $(ADDFLAGS) ./tool/tbot/... ./lib/tbot/... \
		| tee $(TEST_LOG_DIR)/unit.json \
		| gotestsum --raw-command -- cat

# Make sure untagged touchid code build/tests.
.PHONY: test-go-touch-id
test-go-touch-id: FLAGS ?= -race -shuffle on
test-go-touch-id: SUBJECT ?= ./lib/auth/touchid/...
test-go-touch-id:
ifneq ("$(TOUCHID_TAG)", "")
	$(CGOFLAG) go test -cover -json $(PACKAGES) $(SUBJECT) $(FLAGS) $(ADDFLAGS) \
		| tee $(TEST_LOG_DIR)/unit.json \
		| gotestsum --raw-command -- cat
endif

# Runs benchmarks once to make sure they pass.
# This is intended to run in CI during unit testing to make sure benchmarks don't break.
# To limit noise and improve speed this will only run on packages that have benchmarks.
# Race detection is not enabled because it significantly slows down benchmarks.
# todo: Use gotestsum when it is compatible with benchmark output. Currently will consider all benchmarks failed.
.PHONY: test-go-bench
test-go-bench: PACKAGES = $(shell grep --exclude-dir api --include "*_test.go" -lr testing.B .  | xargs dirname | xargs go list | sort -u)
test-go-bench: BENCHMARK_SKIP_PATTERN = "^BenchmarkRoot"
test-go-bench: | $(TEST_LOG_DIR)
	go test -run ^$$ -bench . -skip $(BENCHMARK_SKIP_PATTERN) -benchtime 1x $(PACKAGES) \
		| tee $(TEST_LOG_DIR)/bench.txt

test-go-bench-root: PACKAGES = $(shell grep --exclude-dir api --include "*_test.go" -lr BenchmarkRoot .  | xargs dirname | xargs go list | sort -u)
test-go-bench-root: BENCHMARK_PATTERN = "^BenchmarkRoot"
test-go-bench-root: BENCHMARK_SKIP_PATTERN = ""
test-go-bench-root: | $(TEST_LOG_DIR)
	go test -run ^$$ -bench $(BENCHMARK_PATTERN) -skip $(BENCHMARK_SKIP_PATTERN) -benchtime 1x $(PACKAGES) \
		| tee $(TEST_LOG_DIR)/bench.txt

# Make sure untagged vnetdaemon code build/tests.
.PHONY: test-go-vnet-daemon
test-go-vnet-daemon: FLAGS ?= -race -shuffle on
test-go-vnet-daemon: SUBJECT ?= ./lib/vnet/daemon/...
test-go-vnet-daemon:
ifneq ("$(VNETDAEMON_TAG)", "")
	$(CGOFLAG) go test -cover -json $(PACKAGES) $(SUBJECT) $(FLAGS) $(ADDFLAGS) \
		| tee $(TEST_LOG_DIR)/unit.json \
		| gotestsum --raw-command -- cat
endif

# Runs ci tsh tests
.PHONY: test-go-tsh
test-go-tsh: FLAGS ?= -race -shuffle on
test-go-tsh: SUBJECT ?= github.com/gravitational/teleport/tool/tsh/...
test-go-tsh:
	$(CGOFLAG_TSH) go test -cover -json -tags "$(PAM_TAG) $(FIPS_TAG) $(LIBFIDO2_TEST_TAG) $(TOUCHID_TAG) $(PIV_TEST_TAG) $(VNETDAEMON_TAG)" $(PACKAGES) $(SUBJECT) $(FLAGS) $(ADDFLAGS) \
		| tee $(TEST_LOG_DIR)/unit.json \
		| gotestsum --raw-command -- cat

# Chaos tests have high concurrency, run without race detector and have TestChaos prefix.
.PHONY: test-go-chaos
test-go-chaos: CHAOS_FOLDERS = $(shell find . -type f -name '*chaos*.go' | xargs dirname | uniq)
test-go-chaos:
	$(CGOFLAG) go test -cover -json -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" -test.run=TestChaos $(CHAOS_FOLDERS) \
		| tee $(TEST_LOG_DIR)/chaos.json \
		| gotestsum --raw-command -- cat

#
# Runs all Go tests except integration, end-to-end, and chaos, called by CI/CD.
#
UNIT_ROOT_REGEX := ^TestRoot
.PHONY: test-go-root
test-go-root: ensure-webassets bpf-bytecode rdpclient $(TEST_LOG_DIR) ensure-gotestsum
test-go-root: FLAGS ?= -race -shuffle on
test-go-root: PACKAGES = $(shell go list $(ADDFLAGS) ./... | grep -v -e e2e -e integration -e integrations/operator)
test-go-root: $(VERSRC)
	$(CGOFLAG) go test -json -run "$(UNIT_ROOT_REGEX)" -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" $(PACKAGES) $(FLAGS) $(ADDFLAGS) \
		| tee $(TEST_LOG_DIR)/unit-root.json \
		| gotestsum --raw-command -- cat

#
# Runs Go tests on the api module. These have to be run separately as the package name is different.
#
.PHONY: test-api
test-api: $(VERSRC) $(TEST_LOG_DIR) ensure-gotestsum
test-api: FLAGS ?= -race -shuffle on
test-api: SUBJECT ?= $(shell cd api && go list ./...)
test-api:
	cd api && $(CGOFLAG) go test -json -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" $(PACKAGES) $(SUBJECT) $(FLAGS) $(ADDFLAGS) \
		| tee $(TEST_LOG_DIR)/api.json \
		| gotestsum --raw-command -- cat

#
# Runs Teleport Operator tests.
# We have to run them using the makefile to ensure the installation of the k8s test tools (envtest)
#
.PHONY: test-operator
test-operator:
	make -C integrations/operator test
#
# Runs Teleport Terraform provider tests.
#
.PHONY: test-terraform-provider
test-terraform-provider:
	make -C integrations test-terraform-provider
#
# Runs Go tests on the integrations/kube-agent-updater module. These have to be run separately as the package name is different.
#
.PHONY: test-kube-agent-updater
test-kube-agent-updater: $(VERSRC) $(TEST_LOG_DIR) ensure-gotestsum
test-kube-agent-updater: FLAGS ?= -race -shuffle on
test-kube-agent-updater: SUBJECT ?= $(shell cd integrations/kube-agent-updater && go list ./...)
test-kube-agent-updater:
	cd integrations/kube-agent-updater && $(CGOFLAG) go test -json -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" $(PACKAGES) $(SUBJECT) $(FLAGS) $(ADDFLAGS) \
		| tee $(TEST_LOG_DIR)/kube-agent-updater.json \
		| gotestsum --raw-command --format=testname -- cat

.PHONY: test-access-integrations
test-access-integrations:
	make -C integrations test-access

.PHONY: test-event-handler-integrations
test-event-handler-integrations:
	make -C integrations test-event-handler

.PHONY: test-integrations-lib
test-integrations-lib:
	make -C integrations test-lib

#
# Runs Go tests on the examples/teleport-usage module. These have to be run separately as the package name is different.
#
.PHONY: test-teleport-usage
test-teleport-usage: $(VERSRC) $(TEST_LOG_DIR) ensure-gotestsum
test-teleport-usage: FLAGS ?= -race -shuffle on
test-teleport-usage: SUBJECT ?= $(shell cd examples/teleport-usage && go list ./...)
test-teleport-usage:
	cd examples/teleport-usage && $(CGOFLAG) go test -json -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" $(PACKAGES) $(SUBJECT) $(FLAGS) $(ADDFLAGS) \
		| tee $(TEST_LOG_DIR)/teleport-usage.json \
		| gotestsum --raw-command -- cat

#
# Flaky test detection. Usually run from CI nightly, overriding these default parameters
# This runs the same tests as test-go-unit but repeatedly to try to detect flaky tests.
#
# TODO(jakule): Migrate to gotestsum
.PHONY: test-go-flaky
FLAKY_RUNS ?= 3
FLAKY_TIMEOUT ?= 1h
FLAKY_TOP_N ?= 20
FLAKY_SUMMARY_FILE ?= /tmp/flaky-report.txt
test-go-flaky: FLAGS ?= -race -shuffle on
test-go-flaky: SUBJECT ?= $(shell go list ./... | grep -v -e e2e -e integration -e tool/tsh -e integrations/operator -e integrations/access -e integrations/lib )
test-go-flaky: GO_BUILD_TAGS ?= $(PAM_TAG) $(FIPS_TAG) $(BPF_TAG) $(TOUCHID_TAG) $(PIV_TEST_TAG) $(VNETDAEMON_TAG)
test-go-flaky: RENDER_FLAGS ?= -report-by flakiness -summary-file $(FLAKY_SUMMARY_FILE) -top $(FLAKY_TOP_N)
test-go-flaky: test-go-prepare $(RENDER_TESTS) $(RERUN)
	$(CGOFLAG) $(RERUN) -n $(FLAKY_RUNS) -t $(FLAKY_TIMEOUT) \
		go test -count=1 -cover -json -tags "$(GO_BUILD_TAGS)" $(SUBJECT) $(FLAGS) $(ADDFLAGS) \
		| $(RENDER_TESTS) $(RENDER_FLAGS)

#
# Runs cargo test on our Rust modules.
# (a no-op if cargo and rustc are not installed)
#
ifneq ($(CHECK_RUST),)
ifneq ($(CHECK_CARGO),)
.PHONY: test-rust
test-rust:
	cargo test
else
.PHONY: test-rust
test-rust:
endif
endif

# Run all shell script unit tests (using https://github.com/bats-core/bats-core)
.PHONY: test-sh
test-sh:
	@if ! type bats 2>&1 >/dev/null; then \
		echo "Not running 'test-sh' target as 'bats' is not installed."; \
		if [ "$${CI}" = "true" ]; then echo "This is a failure when running in CI." && exit 1; fi; \
		exit 0; \
	fi; \
	bats $(BATSFLAGS) ./assets/aws/files/tests


.PHONY: test-e2e
test-e2e:
	make -C e2e test

.PHONY: run-etcd
run-etcd:
	docker build -f .github/services/Dockerfile.etcd -t etcdbox --build-arg=ETCD_VERSION=3.5.9 .
	docker run -it --rm -p'2379:2379' etcdbox

#
# Integration tests. Need a TTY to work.
# Any tests which need to run as root must be skipped during regular integration testing.
#
.PHONY: integration
integration: FLAGS ?= -v -race
integration: PACKAGES = $(shell go list ./... | grep 'integration\([^s]\|$$\)' | grep -v integrations/lib/testing/integration )
integration:  $(TEST_LOG_DIR) ensure-gotestsum
	@echo KUBECONFIG is: $(KUBECONFIG), TEST_KUBE: $(TEST_KUBE)
	$(CGOFLAG) go test -timeout 30m -json -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" $(PACKAGES) $(FLAGS) \
		| tee $(TEST_LOG_DIR)/integration.json \
		| gotestsum --raw-command --format=testname -- cat

#
# Integration tests that run Kubernetes tests in order to complete successfully
# are run separately to all other integration tests.
#
INTEGRATION_KUBE_REGEX := TestKube.*
.PHONY: integration-kube
integration-kube: FLAGS ?= -v -race
integration-kube: PACKAGES = $(shell go list ./... | grep 'integration\([^s]\|$$\)')
integration-kube: $(TEST_LOG_DIR) ensure-gotestsum
	@echo KUBECONFIG is: $(KUBECONFIG), TEST_KUBE: $(TEST_KUBE)
	$(CGOFLAG) go test -json -run "$(INTEGRATION_KUBE_REGEX)" $(PACKAGES) $(FLAGS) \
		| tee $(TEST_LOG_DIR)/integration-kube.json \
		| gotestsum --raw-command --format=testname -- cat

#
# Integration tests which need to be run as root in order to complete successfully
# are run separately to all other integration tests. Need a TTY to work.
#
INTEGRATION_ROOT_REGEX := ^TestRoot
.PHONY: integration-root
integration-root: FLAGS ?= -v -race
integration-root: PACKAGES = $(shell go list ./... | grep 'integration\([^s]\|$$\)')
integration-root: $(TEST_LOG_DIR) ensure-gotestsum
	$(CGOFLAG) go test -json -run "$(INTEGRATION_ROOT_REGEX)" $(PACKAGES) $(FLAGS) \
		| tee $(TEST_LOG_DIR)/integration-root.json \
		| gotestsum --raw-command --format=testname -- cat


.PHONY: e2e-aws
e2e-aws: FLAGS ?= -v -race
e2e-aws: PACKAGES = $(shell go list ./... | grep 'e2e/aws')
e2e-aws: $(TEST_LOG_DIR) ensure-gotestsum
	@echo TEST_KUBE: $(TEST_KUBE) TEST_AWS_DB: $(TEST_AWS_DB)
	$(CGOFLAG) go test -json $(PACKAGES) $(FLAGS) $(ADDFLAGS)\
		| tee $(TEST_LOG_DIR)/e2e-aws.json \
		| gotestsum --raw-command --format=testname -- cat

#
# Lint the source code.
# By default lint scans the entire repo. Pass GO_LINT_FLAGS='--new' to only scan local
# changes (or last commit).
#
.PHONY: lint
lint: lint-api lint-go lint-kube-agent-updater lint-tools lint-protos lint-no-actions

#
# Runs linters without dedicated GitHub Actions.
#
.PHONY: lint-no-actions
lint-no-actions: lint-sh lint-license

.PHONY: lint-tools
lint-tools: lint-build-tooling lint-backport

#
# Runs the clippy linter and rustfmt on our rust modules
# (a no-op if cargo and rustc are not installed)
#
ifneq ($(CHECK_RUST),)
ifneq ($(CHECK_CARGO),)
.PHONY: lint-rust
lint-rust:
	cargo clippy --locked --all-targets -- -D warnings \
		&& cargo fmt -- --check
else
.PHONY: lint-rust
lint-rust:
endif
endif

.PHONY: lint-go
lint-go: GO_LINT_FLAGS ?=
lint-go:
	golangci-lint run -c .golangci.yml --build-tags='$(LIBFIDO2_TEST_TAG) $(TOUCHID_TAG) $(PIV_LINT_TAG) $(VNETDAEMON_TAG)' $(GO_LINT_FLAGS)
	$(MAKE) -C integrations/terraform lint
	$(MAKE) -C integrations/event-handler lint

.PHONY: fix-imports
fix-imports:
ifndef TELEPORT_DEVBOX
	$(MAKE) -C build.assets/ fix-imports
else
	$(MAKE) fix-imports/host
endif

.PHONY: fix-imports/host
fix-imports/host:
	@if ! type gci >/dev/null 2>&1; then\
		echo 'gci is not installed or is missing from PATH, consider installing it ("go install github.com/daixiang0/gci@latest") or use "make -C build.assets/ fix-imports"';\
		exit 1;\
	fi
	gci write -s standard -s default  -s 'prefix(github.com/gravitational/teleport)' -s 'prefix(github.com/gravitational/teleport/integrations/terraform,github.com/gravitational/teleport/integrations/event-handler)' --skip-generated .

lint-build-tooling: GO_LINT_FLAGS ?=
lint-build-tooling:
	cd build.assets/tooling && golangci-lint run -c ../../.golangci.yml $(GO_LINT_FLAGS)

.PHONY: lint-backport
lint-backport: GO_LINT_FLAGS ?=
lint-backport:
	cd assets/backport && golangci-lint run -c ../../.golangci.yml $(GO_LINT_FLAGS)

# api is no longer part of the teleport package, so golangci-lint skips it by default
.PHONY: lint-api
lint-api: GO_LINT_API_FLAGS ?=
lint-api:
	cd api && golangci-lint run -c ../.golangci.yml $(GO_LINT_API_FLAGS)

.PHONY: lint-kube-agent-updater
lint-kube-agent-updater: GO_LINT_API_FLAGS ?=
lint-kube-agent-updater:
	cd integrations/kube-agent-updater && golangci-lint run -c ../../.golangci.yml $(GO_LINT_API_FLAGS)

# TODO(awly): remove the `--exclude` flag after cleaning up existing scripts
.PHONY: lint-sh
lint-sh: SH_LINT_FLAGS ?=
lint-sh:
	find . -type f \( -name '*.sh' -or -name '*.sh.tmpl' \) -not -path "*/node_modules/*" | xargs \
		shellcheck \
		--exclude=SC2086 \
		--exclude=SC1091 \
		$(SH_LINT_FLAGS)

	# lint AWS AMI scripts
	# SC1091 prints errors when "source" directives are not followed
	find assets/aws/files/bin -type f | xargs \
		shellcheck \
		--exclude=SC2086 \
		--exclude=SC1091 \
		--exclude=SC2129 \
		$(SH_LINT_FLAGS)

# Lints all the Helm charts found in directories under examples/chart and exits on failure
# If there is a .lint directory inside, the chart gets linted once for each .yaml file in that directory
# We inherit yamllint's 'relaxed' configuration as it's more compatible with Helm output and will only error on
# show-stopping issues. Kubernetes' YAML parser is not particularly fussy.
# If errors are found, the file is printed with line numbers to aid in debugging.
.PHONY: lint-helm
lint-helm:
	@if ! type yamllint 2>&1 >/dev/null; then \
		echo "Not running 'lint-helm' target as 'yamllint' is not installed."; \
		if [ "$${CI}" = "true" ]; then echo "This is a failure when running in CI." && exit 1; fi; \
		exit 0; \
	fi; \
	for CHART in ./examples/chart/teleport-cluster ./examples/chart/teleport-kube-agent ./examples/chart/teleport-cluster/charts/teleport-operator ./examples/chart/tbot; do \
		if [ -d $${CHART}/.lint ]; then \
			for VALUES in $${CHART}/.lint/*.yaml; do \
				export HELM_TEMP=$$(mktemp); \
				echo -n "Using values from '$${VALUES}': "; \
				yamllint -c examples/chart/.lint-config.yaml $${VALUES} || { cat -en $${VALUES}; exit 1; }; \
				helm lint --quiet --strict $${CHART} -f $${VALUES} || exit 1; \
				helm template test $${CHART} -f $${VALUES} 1>$${HELM_TEMP} || exit 1; \
				yamllint -c examples/chart/.lint-config.yaml $${HELM_TEMP} || { cat -en $${HELM_TEMP}; exit 1; }; \
			done \
		else \
			export HELM_TEMP=$$(mktemp); \
			helm lint --quiet --strict $${CHART} || exit 1; \
			helm template test $${CHART} 1>$${HELM_TEMP} || exit 1; \
			yamllint -c examples/chart/.lint-config.yaml $${HELM_TEMP} || { cat -en $${HELM_TEMP}; exit 1; }; \
		fi; \
	done
	$(MAKE) -C examples/chart check-chart-ref

ADDLICENSE_COMMON_ARGS := -c 'Gravitational, Inc.' \
		-ignore '**/*.c' \
		-ignore '**/*.h' \
		-ignore '**/*.html' \
		-ignore '**/*.js' \
		-ignore '**/*.py' \
		-ignore '**/*.sh' \
		-ignore '**/*.tf' \
		-ignore '**/*.yaml' \
		-ignore '**/*.yml' \
		-ignore '**/*.sql' \
		-ignore '**/Dockerfile' \
		-ignore 'api/version.go' \
		-ignore 'docs/pages/includes/**/*.go' \
		-ignore 'e/**' \
		-ignore 'gen/**' \
		-ignore 'gitref.go' \
		-ignore 'lib/srv/desktop/rdp/rdpclient/target/**' \
		-ignore 'lib/web/build/**' \
		-ignore 'version.go' \
		-ignore 'webassets/**' \
		-ignore '**/node_modules/**' \
		-ignore 'web/packages/design/src/assets/icomoon/style.css' \
		-ignore '**/.terraform.lock.hcl' \
		-ignore 'ignoreme'
ADDLICENSE_AGPL3_ARGS := $(ADDLICENSE_COMMON_ARGS) \
		-ignore 'api/**' \
		-f $(CURDIR)/build.assets/LICENSE.header
ADDLICENSE_APACHE2_ARGS := $(ADDLICENSE_COMMON_ARGS) \
		-l apache

ADDLICENSE = go run github.com/google/addlicense@v1.0.0

.PHONY: lint-license
lint-license:
	$(ADDLICENSE) $(ADDLICENSE_AGPL3_ARGS) -check * 2>/dev/null
	$(ADDLICENSE) $(ADDLICENSE_APACHE2_ARGS) -check api/* 2>/dev/null

.PHONY: fix-license
fix-license:
	$(ADDLICENSE) $(ADDLICENSE_AGPL3_ARGS) * 2>/dev/null
	$(ADDLICENSE) $(ADDLICENSE_APACHE2_ARGS) api/* 2>/dev/null

# This rule updates version files and Helm snapshots based on the Makefile
# VERSION variable.
#
# Used prior to a release by bumping VERSION in this Makefile and then
# running "make update-version".
.PHONY: update-version
update-version: version test-helm-update-snapshots

# This rule triggers re-generation of version files if Makefile changes.
.PHONY: version
version: $(VERSRC)

# This rule triggers re-generation of version files specified if Makefile changes.
$(VERSRC): Makefile
	VERSION=$(VERSION) $(MAKE) -f version.mk setver

# Pushes GITTAG and api/GITTAG to GitHub.
#
# Before running `make update-tag`, do:
#
# 1. Commit your changes
# 2. Bump VERSION variable (eg, "vMAJOR.(MINOR+1).0-dev-$USER.1")
# 3. Run `make update-version`
# 4. Commit version changes to git
# 5. Make sure it all builds (`make release` or equivalent)
# 6. Run `make update-tag` to tag repos with $(VERSION)
# 7. Run `make tag-build` to build the tag on GitHub Actions
# 8. Run `make tag-publish` after `make-build` tag has completed to
#    publish the built artifacts.
#
# GHA tag builds: https://github.com/gravitational/teleport.e/actions/workflows/tag-build.yaml
# GHA tag publish: https://github.com/gravitational/teleport.e/actions/workflows/tag-publish.yaml
.PHONY: update-tag
update-tag: TAG_REMOTE ?= origin
update-tag:
	@test $(VERSION)
	cd build.assets/tooling && CGO_ENABLED=0 go run ./cmd/check -check valid -tag $(GITTAG)
	git tag $(GITTAG)
	git tag api/$(GITTAG)
	(cd e && git tag $(GITTAG) && git push origin $(GITTAG))
	git push $(TAG_REMOTE) $(GITTAG) && git push $(TAG_REMOTE) api/$(GITTAG)

# find-any evaluates to non-empty (true) if any of the strings in $(1) are contained in $(2)
# e.g.
#   $(call find-any,-cloud -dev,1.2.3-dev.1) == true
#   $(call find-any,-cloud -dev,1.2.3-cloud.1) == true
#   $(call find-any,-cloud -dev,1.2.3) == false
find-any = $(strip $(foreach str,$(1),$(findstring $(str),$(2))))

# IS_CLOUD_SEMVER is non-empty if $(VERSION) contains a cloud-only pre-release tag,
# and is empty if not.
CLOUD_VERSIONS = -cloud. -dev.cloud.
IS_CLOUD_SEMVER = $(call find-any,$(CLOUD_VERSIONS),$(VERSION))

# IS_PROD_SEMVER is non-empty if $(VERSION) does not contains a pre-release component, or
# if it does, it is -cloud.
PROD_VERSIONS = -cloud.
IS_PROD_SEMVER = $(if $(findstring -,$(VERSION)),$(call find-any,$(PROD_VERSIONS),$(VERSION)),true)

# Builds a tag build on GitHub Actions.
# Starts a tag publish run using e/.github/workflows/tag-build.yaml
# for the tag v$(VERSION).
# If the $(VERSION) variable contains a cloud pre-release component, -cloud. or
# -dev.cloud., then the tag-build workflow is run with `cloud-only=true`. This can be
# specified explicitly with `make tag-build CLOUD_ONLY=<true|false>`.
.PHONY: tag-build
tag-build: CLOUD_ONLY = $(if $(IS_CLOUD_SEMVER),true,false)
tag-build: ENVIRONMENT = $(if $(IS_PROD_SEMVER),prod/build,stage/build)
tag-build:
	@which gh >/dev/null 2>&1 || { echo 'gh command needed. https://github.com/cli/cli'; exit 1; }
	gh workflow run tag-build.yaml \
		--repo gravitational/teleport.e \
		--ref "v$(VERSION)" \
		-f "oss-teleport-repo=$(shell gh repo view --json nameWithOwner --jq .nameWithOwner)" \
		-f "oss-teleport-ref=v$(VERSION)" \
		-f "cloud-only=$(CLOUD_ONLY)" \
		-f "environment=$(ENVIRONMENT)"
	@echo See runs at: https://github.com/gravitational/teleport.e/actions/workflows/tag-build.yaml

# Publishes a tag build.
# Starts a tag publish run using e/.github/workflows/tag-publish.yaml
# for the tag v$(VERSION).
# If the $(VERSION) variable contains a cloud pre-release component, -cloud. or
# -dev.cloud., then the tag-publish workflow is run with `cloud-only=true`. This can be
# specified explicitly with `make tag-publish CLOUD_ONLY=<true|false>`.
.PHONY: tag-publish
tag-publish: CLOUD_ONLY = $(if $(IS_CLOUD_SEMVER),true,false)
tag-publish: ENVIRONMENT = $(if $(IS_PROD_SEMVER),prod/publish,stage/publish)
tag-publish:
	@which gh >/dev/null 2>&1 || { echo 'gh command needed. https://github.com/cli/cli'; exit 1; }
	gh workflow run tag-publish.yaml \
		--repo gravitational/teleport.e \
		--ref "v$(VERSION)" \
		-f "oss-teleport-repo=$(shell gh repo view --json nameWithOwner --jq .nameWithOwner)" \
		-f "oss-teleport-ref=v$(VERSION)" \
		-f "cloud-only=$(CLOUD_ONLY)" \
		-f "environment=$(ENVIRONMENT)"
	@echo See runs at: https://github.com/gravitational/teleport.e/actions/workflows/tag-publish.yaml

.PHONY: test-package
test-package: remove-temp-files
	go test -v ./$(p)

.PHONY: test-grep-package
test-grep-package: remove-temp-files
	go test -v ./$(p) -check.f=$(e)

.PHONY: cover-package
cover-package: remove-temp-files
	go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

.PHONY: profile
profile:
	go tool pprof http://localhost:6060/debug/pprof/profile

.PHONY: sloccount
sloccount:
	find . -o -name "*.go" -print0 | xargs -0 wc -l

#
# print-go-version outputs Go version as a semver without "go" prefix
#
.PHONY: print-go-version
print-go-version:
	@$(MAKE) -C build.assets print-go-version | sed "s/go//"

# Dockerized build: useful for making Linux releases on macOS
.PHONY:docker
docker:
	make -C build.assets build

# Dockerized build: useful for making Linux binaries on macOS
.PHONY:docker-binaries
docker-binaries: clean
	make -C build.assets build-binaries PIV=$(PIV)

# Interactively enters a Docker container (which you can build and run Teleport inside of)
.PHONY:enter
enter:
	make -C build.assets enter

# Interactively enters a Docker container, as root (which you can build and run Teleport inside of)
.PHONY:enter-root
enter-root:
	make -C build.assets enter-root

# Interactively enters a Docker container (which you can build and run Teleport inside of).
# Similar to `enter`, but uses the centos7 container.
.PHONY:enter/centos7
enter/centos7:
	make -C build.assets enter/centos7

.PHONY:enter/centos7-fips
enter/centos7-fips:
	make -C build.assets enter/centos7-fips

.PHONY:enter/grpcbox
enter/grpcbox:
	make -C build.assets enter/grpcbox

.PHONY:enter/node
enter/node:
	make -C build.assets enter/node

.PHONY:enter/arm
enter/arm:
	make -C build.assets enter/arm

BUF := buf

# protos/all runs build, lint and format on all protos.
# Use `make grpc` to regenerate protos inside buildbox.
.PHONY: protos/all
protos/all: protos/build protos/lint protos/format

.PHONY: protos/build
protos/build: buf/installed
	$(BUF) build

.PHONY: protos/format
protos/format: buf/installed
	$(BUF) format -w

.PHONY: protos/lint
protos/lint: buf/installed
	$(BUF) lint
	$(BUF) lint --config=buf-legacy.yaml api/proto

.PHONY: protos/breaking
protos/breaking: BASE=origin/master
protos/breaking: buf/installed
	@echo Checking compatibility against BASE=$(BASE)
	buf breaking . --against '.git#branch=$(BASE)'

.PHONY: lint-protos
lint-protos: protos/lint

.PHONY: lint-breaking
lint-breaking: protos/breaking

.PHONY: buf/installed
buf/installed:
	@if ! type -p $(BUF) >/dev/null; then \
		echo 'Buf is required to build/format/lint protos. Follow https://docs.buf.build/installation.'; \
		exit 1; \
	fi

GODERIVE := $(TOOLINGDIR)/bin/goderive
# derive will generate derived functions for our API.
# we need to build goderive first otherwise it will not be able to resolve dependencies
# in the api/types/discoveryconfig package
.PHONY: derive
derive:
	cd $(TOOLINGDIR) && go build -o $(GODERIVE) ./cmd/goderive/main.go
	$(GODERIVE) ./api/types ./api/types/discoveryconfig

# derive-up-to-date checks if the generated derived functions are up to date.
.PHONY: derive-up-to-date
derive-up-to-date: must-start-clean/host derive
	@if ! git diff --quiet; then \
		./build.assets/please-run.sh "derived functions" "make derive"; \
		exit 1; \
	fi

# grpc generates gRPC stubs from service definitions.
# This target runs in the buildbox container.
.PHONY: grpc
grpc:
ifndef TELEPORT_DEVBOX
	$(MAKE) -C build.assets grpc
else
	$(MAKE) grpc/host
endif

# grpc/host generates gRPC stubs.
# Unlike grpc, this target runs locally.
.PHONY: grpc/host
grpc/host: protos/all
	@build.assets/genproto.sh

# protos-up-to-date checks if the generated gRPC stubs are up to date.
# This target runs in the buildbox container.
.PHONY: protos-up-to-date
protos-up-to-date:
ifndef TELEPORT_DEVBOX
	$(MAKE) -C build.assets protos-up-to-date
else
	$(MAKE) protos-up-to-date/host
endif

# protos-up-to-date/host checks if the generated gRPC stubs are up to date.
# Unlike protos-up-to-date, this target runs locally.
.PHONY: protos-up-to-date/host
protos-up-to-date/host: must-start-clean/host grpc/host
	@if ! git diff --quiet; then \
		./build.assets/please-run.sh "protos gRPC" "make grpc"; \
		exit 1; \
	fi

.PHONY: must-start-clean/host
must-start-clean/host:
	@if ! git diff --quiet; then \
		@echo 'This must be run from a repo with no unstaged commits.'; \
		git diff; \
		exit 1; \
	fi

# crds-up-to-date checks if the generated CRDs from the protobuf stubs are up to date.
.PHONY: crds-up-to-date
crds-up-to-date: must-start-clean/host
	$(MAKE) -C integrations/operator manifests
	@if ! git diff --quiet; then \
		./build.assets/please-run.sh "operator CRD manifests" "make -C integrations/operator crd"; \
		exit 1; \
	fi
	$(MAKE) -C integrations/operator crd-docs
	@if ! git diff --quiet; then \
		./build.assets/please-run.sh "operator CRD docs" "make -C integrations/operator crd"; \
		exit 1; \
	fi
	$(MAKE) -C integrations/operator crd-docs
	@if ! git diff --quiet; then \
		echo 'Please run make -C integrations/operator crd-docs.'; \
		git diff; \
		exit 1; \
	fi

# tfdocs-up-to-date checks if the generated Terraform types and documentation from the protobuf stubs are up to date.
.PHONY: terraform-resources-up-to-date
terraform-resources-up-to-date: must-start-clean/host
	$(MAKE) -C integrations/terraform docs
	@if ! git diff --quiet; then \
		./build.assets/please-run.sh "TF provider docs" "make -C integrations/terraform docs"; \
		exit 1; \
	fi

print/env:
	env

# make install will installs system-wide teleport
.PHONY: install
install: build
	@echo "\n** Make sure to run 'make install' as root! **\n"
	cp -f $(BUILDDIR)/tctl      $(BINDIR)/
	cp -f $(BUILDDIR)/tsh       $(BINDIR)/
	cp -f $(BUILDDIR)/tbot      $(BINDIR)/
	cp -f $(BUILDDIR)/teleport  $(BINDIR)/
	mkdir -p $(DATADIR)

# Docker image build. Always build the binaries themselves within docker (see
# the "docker" rule) to avoid dependencies on the host libc version.
.PHONY: image
image: OS=linux
image: TARBALL_PATH_SECTION:=-s "$(shell pwd)"
image: clean docker-binaries build-archive oss-deb
	cp ./build.assets/charts/Dockerfile $(BUILDDIR)/
	cd $(BUILDDIR) && docker build --no-cache . -t $(DOCKER_IMAGE):$(VERSION)-$(ARCH) --target teleport \
		--build-arg DEB_PATH="./teleport_$(VERSION)_$(ARCH).deb"
	if [ -f e/Makefile ]; then $(MAKE) -C e image PIV=$(PIV); fi

.PHONY: print-version
print-version:
	@echo $(VERSION)

.PHONY: chart-ent
chart-ent:
	$(MAKE) -C e chart

RUNTIME_SECTION ?=
TARBALL_PATH_SECTION ?=

ifneq ("$(RUNTIME)", "")
	RUNTIME_SECTION := -r $(RUNTIME)
endif
ifneq ("$(OSS_TARBALL_PATH)", "")
	TARBALL_PATH_SECTION := -s $(OSS_TARBALL_PATH)
endif

# build .pkg
.PHONY: pkg
pkg: | $(RELEASE_DIR)
	mkdir -p $(BUILDDIR)/
	cp ./build.assets/build-package.sh ./build.assets/build-common.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	# runtime is currently ignored on OS X
	# we pass it through for consistency - it will be dropped by the build script
	cd $(BUILDDIR) && ./build-package.sh -t oss -v $(VERSION) -p pkg -b $(TELEPORT_BUNDLEID) -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)
	cp $(BUILDDIR)/teleport-*.pkg $(RELEASE_DIR)
	if [ -f e/Makefile ]; then $(MAKE) -C e pkg; fi

# build tsh client-only .pkg
.PHONY: pkg-tsh
pkg-tsh: | $(RELEASE_DIR)
	./build.assets/build-pkg-tsh.sh -t oss -v $(VERSION) -b $(TSH_BUNDLEID) -a $(ARCH) $(TARBALL_PATH_SECTION)
	mkdir -p $(BUILDDIR)/
	mv tsh*.pkg* $(BUILDDIR)/
	cp $(BUILDDIR)/tsh-*.pkg $(RELEASE_DIR)

# build .rpm
.PHONY: rpm
rpm:
	mkdir -p $(BUILDDIR)/
	cp ./build.assets/build-package.sh ./build.assets/build-common.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	cp -a ./build.assets/rpm $(BUILDDIR)/
	cp -a ./build.assets/rpm-sign $(BUILDDIR)/
	cd $(BUILDDIR) && ./build-package.sh -t oss -v $(VERSION) -p rpm -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)
	if [ -f e/Makefile ]; then $(MAKE) -C e rpm; fi

# build unsigned .rpm (for testing)
.PHONY: rpm-unsigned
rpm-unsigned:
	$(MAKE) UNSIGNED_RPM=true rpm

# build open source .deb only
.PHONY: oss-deb
oss-deb:
	mkdir -p $(BUILDDIR)/
	cp ./build.assets/build-package.sh ./build.assets/build-common.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	cd $(BUILDDIR) && ./build-package.sh -t oss -v $(VERSION) -p deb -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)

# build .deb
.PHONY: deb
deb: oss-deb
	if [ -f e/Makefile ]; then $(MAKE) -C e deb; fi

# check binary compatibility with different OSes
.PHONY: test-compat
test-compat:
	./build.assets/build-test-compat.sh

.PHONY: ensure-webassets
ensure-webassets:
	@if [[ "${WEBASSETS_SKIP_BUILD}" -eq 1 ]]; then mkdir -p webassets/teleport && mkdir -p webassets/teleport/app && cp web/packages/teleport/index.html webassets/teleport/index.html; \
	else MAKE="$(MAKE)" "$(MAKE_DIR)/build.assets/build-webassets-if-changed.sh" OSS webassets/oss-sha build-ui web; fi

.PHONY: ensure-webassets-e
ensure-webassets-e:
	@if [[ "${WEBASSETS_SKIP_BUILD}" -eq 1 ]]; then mkdir -p webassets/teleport && mkdir -p webassets/e/teleport/app && cp web/packages/teleport/index.html webassets/e/teleport/index.html; \
	else MAKE="$(MAKE)" "$(MAKE_DIR)/build.assets/build-webassets-if-changed.sh" Enterprise webassets/e/e-sha build-ui-e web e/web; fi

# Enables the pnpm package manager if it is not already enabled and Corepack
# is available.
# We check if pnpm is enabled, as the user may not have permission to
# enable it, but it may already be available.
# Enabling it merely installs a shim which then needs to be downloaded before first use.
# Corepack typically prompts before downloading a package manager. We work around that
# by issuing a bogus pnpm call with an env var that skips the prompt.
.PHONY: ensure-js-package-manager
ensure-js-package-manager:
	@if [ -z "$$(COREPACK_ENABLE_DOWNLOAD_PROMPT=0 pnpm -v 2>/dev/null)" ]; then \
		if [ -n "$$(corepack --version 2>/dev/null)" ]; then \
			echo 'Info: pnpm is not enabled via Corepack. Enabling pnpm…'; \
			corepack enable pnpm; \
			echo "pnpm $$(COREPACK_ENABLE_DOWNLOAD_PROMPT=0 pnpm -v)"; \
		else \
			echo 'Error: Corepack is not installed, cannot enable pnpm. See the installation guide https://pnpm.io/installation#using-corepack'; \
			exit 1; \
		fi; \
	fi

.PHONY: init-submodules-e
init-submodules-e:
	git submodule init e
	git submodule update

# backport will automatically create backports for a given PR as long as you have the "gh" tool
# installed locally. To backport, type "make backport PR=1234 TO=branch/1,branch/2".
.PHONY: backport
backport:
	(cd ./assets/backport && go run main.go -pr=$(PR) -to=$(TO))

.PHONY: ensure-js-deps
ensure-js-deps:
	@if [[ "${WEBASSETS_SKIP_BUILD}" -eq 1 ]]; then mkdir -p webassets/teleport && touch webassets/teleport/index.html; \
	else $(MAKE) ensure-js-package-manager && pnpm install --frozen-lockfile; fi

.PHONY: build-ui
build-ui: ensure-js-deps
	@[ "${WEBASSETS_SKIP_BUILD}" -eq 1 ] || pnpm build-ui-oss

.PHONY: build-ui-e
build-ui-e: ensure-js-deps
	@[ "${WEBASSETS_SKIP_BUILD}" -eq 1 ] || pnpm build-ui-e

.PHONY: docker-ui
docker-ui:
	$(MAKE) -C build.assets ui

.PHONY: rustup-set-version
rustup-set-version: RUST_VERSION := $(shell $(MAKE) --no-print-directory -C build.assets print-rust-version)
rustup-set-version:
	rustup override set $(RUST_VERSION)

# rustup-install-target-toolchain ensures the required rust compiler is
# installed to build for $(ARCH)/$(OS) for the version of rust we use, as
# defined in build.assets/Makefile. It assumes that `rustup` is already
# installed for managing the rust toolchain.
.PHONY: rustup-install-target-toolchain
rustup-install-target-toolchain: rustup-set-version
	rustup target add $(RUST_TARGET_ARCH)

# changelog generates PR changelog between the provided base tag and the tip of
# the specified branch.
#
# usage: make changelog
# usage: make changelog BASE_BRANCH=branch/v13 BASE_TAG=v13.2.0
# usage: BASE_BRANCH=branch/v13 BASE_TAG=v13.2.0 make changelog
#
# BASE_BRANCH and BASE_TAG will be automatically determined if not specified.
.PHONY: changelog
changelog:
	@go run github.com/gravitational/shared-workflows/tools/changelog@latest \
		--base-branch="$(BASE_BRANCH)" --base-tag="$(BASE_TAG)" ./

# create-github-release will generate release notes from the CHANGELOG.md and will
# create release notes from them.
#
# usage: make create-github-release
# usage: make create-github-release LATEST=true
#
# If it detects that the first version in CHANGELOG.md
# does not match version set it will fail to create a release. If tag doesn't exist it
# will also fail to create a release.
#
# For more information on release notes generation see:
#   https://github.com/gravitational/shared-workflows/tree/gus/release-notes/tools/release-notes#readme
.PHONY: create-github-release
create-github-release: LATEST = false
create-github-release: GITHUB_RELEASE_LABELS = ""
create-github-release:
	@NOTES=$$( \
		go run github.com/gravitational/shared-workflows/tools/release-notes@latest \
			--labels=$(GITHUB_RELEASE_LABELS) $(VERSION) CHANGELOG.md \
	) && gh release create v$(VERSION) \
	-t "Teleport $(VERSION)" \
	--latest=$(LATEST) \
	--verify-tag \
	-F - <<< "$$NOTES"
