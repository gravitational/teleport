# This makefile usually runs as a sub-make by ../Makefile
# But running regular 'make' and 'make clean' works.
ifneq ("$(wildcard /bin/bash)","")
SHELL := /bin/bash -o pipefail
endif
BUILDDIR ?= build
RELEASE_DIR ?= $(BUILDDIR)/artifacts
TELEPORT_DEBUG ?= false
CGOFLAG ?= CGO_ENABLED=1
GITTAG ?= v$(VERSION)
DOCKER_IMAGE ?= teleport-ent
RELEASE_ENT = teleport-ent-$(GITTAG)-$(OS)-$(ARCH)-bin
BINS = teleport tctl tsh tbot
BINARIES ?= $(addprefix $(BUILDDIR)/,$(BINS))
KUSTOMIZE_NO_DYNAMIC_PLUGIN ?= kustomize_disable_go_plugin_support

# When TELEPORT_DEBUG is true, set flags so produce
# debugger-friendly builds.
ifeq ("$(TELEPORT_DEBUG)","true")
BUILDFLAGS ?= $(ADDFLAGS) -gcflags=all="-N -l $(KUBECTL_SETVERSION)"
else
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s $(LDFLAGS) $(KUBECTL_SETVERSION)' -trimpath -buildmode=pie
endif

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

FIPS_ENT_MESSAGE := without-FIPS-support
# FIPS support must be requested at build time.
ifneq ("$(FIPS)","")
FIPS_TAG := fips
FIPS_ENT_MESSAGE := with-FIPS-support
RELEASE_ENT = teleport-ent-$(GITTAG)-$(OS)-$(ARCH)-fips-bin
# See the comment in tool/modules/fips.go.hide for explanation.
$(shell ln -sf fips.go.hide tool/modules/fips.go)
else
# See the comment in tool/modules/fips.go.hide for explanation.
$(shell rm -f tool/modules/fips.go)
endif

PAM_ENT_MESSAGE := without-PAM-support
# PAM support will only be built into Teleport if headers exist at build time.
ifneq ("$(wildcard /usr/include/security/pam_appl.h)","")
PAM_TAG = pam
PAM_ENT_MESSAGE := with-PAM-support
endif

# darwin universal (Intel + Apple Silicon combined) binary vars
RELEASE_darwin_arm64 = $(RELEASE_DIR)/teleport-ent-$(GITTAG)-darwin-arm64-bin.tar.gz
RELEASE_darwin_amd64 = $(RELEASE_DIR)/teleport-ent-$(GITTAG)-darwin-amd64-bin.tar.gz
BUILDDIR_arm64 = $(BUILDDIR)/arm64
BUILDDIR_amd64 = $(BUILDDIR)/amd64
# TARBINS in the path of the binaries in the release tarballs
TARBINS = $(addprefix teleport-ent/,$(BINS))

# Include common setting from OSS repo.
include ../common.mk

# Check if rust and cargo are installed before compiling
CHECK_CARGO := $(shell cargo --version 2>/dev/null)
CHECK_RUST := $(shell rustc --version 2>/dev/null)

with_rdpclient := no
RDPCLIENT_ENT_MESSAGE := without-Windows-RDP-client

CARGO_TARGET_darwin_amd64 := x86_64-apple-darwin
CARGO_TARGET_darwin_arm64 := aarch64-apple-darwin
CARGO_TARGET_linux_arm := arm-unknown-linux-gnueabihf
CARGO_TARGET_linux_arm64 := aarch64-unknown-linux-gnu
CARGO_TARGET_linux_386 := i686-unknown-linux-gnu
CARGO_TARGET_linux_amd64 := x86_64-unknown-linux-gnu

CARGO_TARGET := --target=${CARGO_TARGET_${OS}_${ARCH}}

# If set to 1, Windows RDP client is not built.
RDPCLIENT_SKIP_BUILD ?= 0
export RDPCLIENT_SKIP_BUILD

ifeq ($(RDPCLIENT_SKIP_BUILD),0)
ifneq ($(CHECK_RUST),)
ifneq ($(CHECK_CARGO),)

is_fips_on_arm64 := no
ifneq ("$(FIPS)","")
ifeq ("$(ARCH)","arm64")
is_fips_on_arm64 := yes
endif
endif

# Do not build RDP client on ARM or 386, or for FIPS builds on arm64.
ifneq ("$(ARCH)","arm")
ifneq ("$(ARCH)","386")
ifneq ("$(is_fips_on_arm64)","yes")
with_rdpclient := yes
RDPCLIENT_ENT_MESSAGE := with-Windows-RDP-client
RDPCLIENT_TAG := desktop_access_rdp
endif
endif
endif

endif
endif
endif

# Build tsh against libfido2?
# Only build if FIDO2=yes, each platform we support must make this decision
# explicitly.
LIBFIDO2_MESSAGE := without-libfido2
ifeq ("$(FIDO2)", "yes")
LIBFIDO2_MESSAGE := with-libfido2
LIBFIDO2_BUILD_TAG := libfido2 libfido2static
endif

# Enable Touch ID builds?
# Only build if TOUCHID=yes to avoid issues when cross-compiling to 'darwin'
# from other systems.
TOUCHID_MESSAGE := without-Touch-ID
ifeq ("$(TOUCHID)", "yes")
TOUCHID_MESSAGE := with-Touch-ID
TOUCHID_TAG := touchid
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

# Enable PIV test packages for testing.
# This test tag should never be used for builds/releases, only tests.
PIV_TEST_TAG := pivtest

# Enable libfido2 for testing?
# Eagerly enable if we detect the package, we want to test as much as possible.
ifeq ("$(shell pkg-config libfido2 2>/dev/null; echo $$?)", "0")
LIBFIDO2_TEST_TAG := libfido2
endif

# Reproducible builds are only available on select targets, and only when OS=linux.
REPRODUCIBLE ?=
ifneq ("$(OS)","linux")
REPRODUCIBLE = no
endif

# On platforms that support reproducible builds, ensure the archive is created in a reproducible manner.
TAR_FLAGS ?=
ifeq ("$(REPRODUCIBLE)","yes")
TAR_FLAGS = --sort=name --owner=root:0 --group=root:0 --mtime='UTC 2015-03-02' --format=gnu
endif

# Joins elements of the list in arg 2 with the given separator.
#   1. Element separator.
#   2. The list.
EMPTY :=
SPACE := $(EMPTY) $(EMPTY)
join-with = $(subst $(SPACE),$1,$(strip $2))

# Separate TAG messages into comma-separated WITH and WITHOUT lists for readability.
COMMA := ,
MESSAGES := $(PAM_ENT_MESSAGE) $(FIPS_ENT_MESSAGE) $(BPF_MESSAGE) $(RDPCLIENT_ENT_MESSAGE) $(LIBFIDO2_MESSAGE) $(TOUCHID_MESSAGE) $(PIV_MESSAGE)
WITH := $(subst -," ",$(call join-with,$(COMMA) ,$(subst with-,,$(filter with-%,$(MESSAGES)))))
WITHOUT := $(subst -," ",$(call join-with,$(COMMA) ,$(subst without-,,$(filter without-%,$(MESSAGES)))))
RELEASE_ENT_MESSAGE := "Building Enterprise with GOOS=$(OS) GOARCH=$(ARCH) with $(WITH) and without $(WITHOUT)."

# Set CGOFLAG and BUILDFLAGS as needed for the OS/ARCH.
ifeq ("$(OS)","linux")
ifeq (, $(filter $(ARCH), amd64 arm64))
# Link static version of libraries (bpf, pcsc) to reduce system dependencies.
CGOFLAG = CGO_ENABLED=1 CGO_CFLAGS="-I/usr/libbpf-${LIBBPF_VER}/include" CGO_LDFLAGS="-Wl,-Bstatic $(STATIC_LIBS) -Wl,-Bdynamic -Wl,--as-needed"
CGOFLAG_TSH = CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-Bstatic $(STATIC_LIBS_TSH) -Wl,-Bdynamic -Wl,--as-needed"

ifeq ("$(ARCH)","arm64")

# If we're not guaranteed to be building natively on an arm64 system,
# then we'll need to configure the cross compiler.
CGOFLAG = $(CGOFLAG) $(if $(IS_NATIVE_BUILD),,CC=aarch64-linux-gnu-gcc)

endif
else ifeq ("$(ARCH)","arm")

# ARM builds need to specify the correct C compiler
CGOFLAG = CGO_ENABLED=1 $(if $(IS_NATIVE_BUILD),,CC=arm-linux-gnueabihf-gcc)
# Add -debugtramp=2 to work around 24 bit CALL/JMP instruction offset.
BUILDFLAGS = $(ADDFLAGS) -ldflags '-extldflags "-Wl,--long-plt" -w -s -debugtramp=2 $(KUBECTL_SETVERSION)' -trimpath -buildmode=pie

endif
endif

ifeq ("$(OS)","darwin")
# Note the minimum version for Apple silicon (ARM64) is 11.0 and will be automatically
# clamped to the value for builds of that architecture
MINIMUM_SUPPORTED_MACOS_VERSION = 10.15
MACOSX_VERSION_MIN_FLAG = -mmacosx-version-min=$(MINIMUM_SUPPORTED_MACOS_VERSION)
CGOFLAG = CGO_ENABLED=1 CGO_CFLAGS=$(MACOSX_VERSION_MIN_FLAG) CGO_LDFLAGS=$(MACOSX_VERSION_MIN_FLAG)
endif

CGOFLAG_TSH ?= $(CGOFLAG)

# All build Enterprise version of teleport binaries and copy
# over tsh, tctl, and tbot from OSS.
.PHONY: all
all: version
	@echo "---> $(RELEASE_ENT_MESSAGE)"
	$(MAKE) $(BINARIES)

# By making these 3 targets below (tsh, tctl and teleport) PHONY we are solving
# several problems:
# * Build will rely on go build internal caching https://golang.org/doc/go1.10 at all times
# * Manual change detection was broken on a large dependency tree
# If you are considering changing this behavior, please consult with dev team first
.PHONY: $(BUILDDIR)/tctl
$(BUILDDIR)/tctl:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -buildvcs=false -tags "$(PAM_TAG) $(FIPS_TAG) $(LIBFIDO2_BUILD_TAG) $(BPF_TAG) $(PIV_BUILD_TAG) $(KUSTOMIZE_NO_DYNAMIC_PLUGIN)" -o $(BUILDDIR)/tctl $(BUILDFLAGS) ../tool/tctl

.PHONY: $(BUILDDIR)/teleport
$(BUILDDIR)/teleport: ensure-webassets-e bpf-bytecode-e teleport-fips-check rdpclient
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -buildvcs=false -tags "webassets_embed webassets_ent $(PAM_TAG) $(FIPS_TAG) $(BPF_TAG) $(WEBASSETS_TAG) $(RDPCLIENT_TAG) $(PIV_BUILD_TAG) $(KUSTOMIZE_NO_DYNAMIC_PLUGIN)" -o $(BUILDDIR)/teleport $(BUILDFLAGS) ./tool/teleport

.PHONY: $(BUILDDIR)/tsh
$(BUILDDIR)/tsh: KUBECTL_VERSION ?= $(shell go run ../build.assets/kubectl-version/main.go)
$(BUILDDIR)/tsh: KUBECTL_SETVERSION=-X k8s.io/component-base/version.gitVersion=$(KUBECTL_VERSION)
$(BUILDDIR)/tsh:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG_TSH) go build -buildvcs=false -tags "$(PAM_TAG) $(FIPS_TAG) $(LIBFIDO2_BUILD_TAG) $(TOUCHID_TAG) $(PIV_BUILD_TAG) $(KUSTOMIZE_NO_DYNAMIC_PLUGIN)" -o $(BUILDDIR)/tsh $(BUILDFLAGS) ../tool/tsh

.PHONY: $(BUILDDIR)/tbot
$(BUILDDIR)/tbot:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -buildvcs=false -tags "$(FIPS_TAG) $(KUSTOMIZE_NO_DYNAMIC_PLUGIN)" -o $(BUILDDIR)/tbot $(BUILDFLAGS) ../tool/tbot

# A target that allows recompiling and restarting Teleport when Go sources
# change. Note: contrary to what the CompileDaemon documentation says, the
# teleport command is executed from within the current directory, and not the
# watched directory.
TELEPORT_ARGS ?= start
.PHONY: teleport-hot-reload
teleport-hot-reload:
	CompileDaemon \
	  --directory=".." \
		--graceful-kill=true \
		--exclude-dir="../.git" \
		--exclude-dir="../build" \
		--exclude-dir="../e/build" \
		--exclude-dir="../e/web/*/node_modules" \
		--exclude-dir="../node_modules" \
		--exclude-dir="../target" \
		--exclude-dir="../web/packages/*/node_modules" \
		--color \
		--log-prefix=false \
		--build="make -C e build/teleport" \
		--command="build/teleport $(TELEPORT_ARGS)"

.PHONY: ensure-webassets-e
ensure-webassets-e:
	$(MAKE) -C ../ ensure-webassets-e

#
# BPF support (IF ENABLED)
#
ifeq ("$(with_bpf)","yes")
.PHONY: bpf-bytecode-e
bpf-bytecode-e:
	$(MAKE) -C ../ bpf-bytecode

else
.PHONY: bpf-bytecode-e
bpf-bytecode-e:
endif

ifeq ("$(OS)-$(with_rdpclient)", "darwin-yes")
# Set the minimum version linker flag for the rust build of rdpclient (and only rdpclient,
# as the flag is invalid for building ironrdp to wasm in the web UI). Also set an env
# var so any C libraries built by this build also target the correct min version.
rdpclient: export RUSTFLAGS = -C link-arg=$(MACOSX_VERSION_MIN_FLAG)
rdpclient: export MACOSX_DEPLOYMENT_TARGET = $(MINIMUM_SUPPORTED_MACOS_VERSION)
endif

.PHONY: rdpclient
rdpclient:
ifeq ("$(with_rdpclient)", "yes")
	cargo build -p rdp-client $(if $(FIPS),--features=fips) --release --locked $(CARGO_TARGET)
	if [[ ! $$(cbindgen --version 2>/dev/null) ]]; then cargo install cbindgen; fi
	cbindgen --quiet --crate rdp-client --output ../lib/srv/desktop/rdp/rdpclient/librdprs.h --lang c ../lib/srv/desktop/rdp/rdpclient/
endif

# If FIPS mode was requested, first compile an extra Teleport binary with symbols included and check that it contains BoringCrypto references.
.PHONY: teleport-fips-check
teleport-fips-check:
	@if [ "$(FIPS)" != "" ]; then \
		echo "FIPS mode requested, pre-compiling and checking binary for BoringCrypto symbols."; \
		GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG) $(WEBASSETS_TAG)" -o $(BUILDDIR)/teleport-unstripped -ldflags "" ./tool/teleport; \
		if go tool nm $(BUILDDIR)/teleport-unstripped 2>&1 | grep boringcrypto; then \
			echo "BoringCrypto appears enabled."; \
		else \
			echo "-- FIPS mode was requested, but the compiled teleport binary does not contain any BoringCrypto symbols. This is an error!"; \
			exit 1; \
		fi; \
		rm -f $(BUILDDIR)/teleport-unstripped; \
	fi

# make clean: cleans the build
.PHONY: clean
clean:
	@echo "---> Cleaning up Enterprise build artifacts."
	rm -rf $(BUILDDIR)
	-cargo clean
	rm -rf *gz

# make deploy-cloud - Produces a docker image and deploys to a cloud
# tenant in the staging cluster.
.PHONY: deploy-cloud
deploy-cloud:
ifneq ($(RELEASE),)
	@echo "---> Deploying a published release to cloud tenant."
	./scripts/deploy-cloud.sh
else
ifneq ($(CLOUD_SKIP_BUILD),)
	@echo "---> Skipping build - use existing teleport binary \"$(BUILDDIR)/teleport\"."
	@if [[ ! -f "$(BUILDDIR)/teleport" ]]; then \
		echo "No teleport binary found!"; \
		exit 1; \
	fi
else
	$(MAKE) -C ../ ensure-webassets-e
	@echo "---> Building teleport binary."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -buildvcs=false -tags "webassets_embed webassets_ent" -o $(BUILDDIR)/teleport $(BUILDFLAGS) ./tool/teleport;
endif
	@echo "---> Building docker image and deploying to cloud tenant."
	./scripts/deploy-cloud.sh
endif

# make deploy-cloud-login - Ensures access to all services required to deploy
# an image to a cloud tenant in the staging cluster.
.PHONY: deploy-cloud-login
deploy-cloud-login:
	./scripts/deploy-cloud-login.sh

#
# make release - Produces a binary release tarball.
#

$(RELEASE_DIR):
	mkdir -p $@

.PHONY:
export
release:
	@echo "---> $(RELEASE_ENT_MESSAGE)"
ifeq ("$(OS)", "darwin")
	$(MAKE) --no-print-directory release-darwin
else
	$(MAKE) --no-print-directory release-unix
endif

# These are aliases used to make build commands uniform.
.PHONY: release-amd64
release-amd64:
	$(MAKE) release ARCH=amd64

.PHONY: release-amd64-fips
release-amd64-fips:
	$(MAKE) release ARCH=amd64 FIPS=yes

.PHONY: release-386
release-386:
	$(MAKE) release ARCH=386

.PHONY: release-arm
release-arm:
	$(MAKE) release ARCH=arm

.PHONY: release-arm64
release-arm64:
	$(MAKE) release ARCH=arm64

.PHONY: release-arm64-fips
release-arm64-fips:
	$(MAKE) release ARCH=arm64 FIPS=yes

include ../darwin-signing.mk

.PHONY: release-darwin-unsigned
release-darwin-unsigned: RELEASE:=$(RELEASE)-unsigned
release-darwin-unsigned: full build-archive

.PHONY: release-darwin
ifneq ($(ARCH),universal)
release-darwin: release-darwin-unsigned
	$(NOTARIZE_BINARIES)
	$(MAKE) build-archive
else
release-darwin: $(RELEASE_darwin_arm64) $(RELEASE_darwin_amd64)
	mkdir -p $(BUILDDIR_arm64) $(BUILDDIR_amd64)
	tar -C $(BUILDDIR_arm64) -xzf $(RELEASE_darwin_arm64) --strip-components=1 $(TARBINS)
	tar -C $(BUILDDIR_amd64) -xzf $(RELEASE_darwin_amd64) --strip-components=1 $(TARBINS)
	lipo -create -output $(BUILDDIR)/teleport $(BUILDDIR_arm64)/teleport $(BUILDDIR_amd64)/teleport
	lipo -create -output $(BUILDDIR)/tctl $(BUILDDIR_arm64)/tctl $(BUILDDIR_amd64)/tctl
	lipo -create -output $(BUILDDIR)/tsh $(BUILDDIR_arm64)/tsh $(BUILDDIR_amd64)/tsh
	lipo -create -output $(BUILDDIR)/tbot $(BUILDDIR_arm64)/tbot $(BUILDDIR_amd64)/tbot
	$(MAKE) ARCH=universal build-archive
endif

#
# make build-archive - Packages the results of a build into a release tarball
#
.PHONY: build-archive
build-archive: | $(RELEASE_DIR)
	@echo "---> Creating Enterprise release archive."
	mkdir teleport-ent
	cp -rf $(BINARIES) \
		../examples \
		../build.assets/install \
		../README.md \
		../CHANGELOG.md \
		teleport-ent/
	echo $(GITTAG) > teleport-ent/VERSION
	tar $(TAR_FLAGS) -c teleport-ent | gzip -n > $(RELEASE_ENT).tar.gz
	cp $(RELEASE_ENT).tar.gz $(RELEASE_DIR)
	# linux-amd64 generates a centos7-compatible archive. Make a copy with the -centos7 label,
	# for the releases page. We should probably drop that at some point.
	$(if $(filter linux-amd64,$(OS)-$(ARCH)), \
		cp $(RELEASE_ENT).tar.gz $(RELEASE_DIR)/$(subst amd64,amd64-centos7,$(RELEASE_ENT)).tar.gz \
	)
	rm -rf teleport-ent
	@echo "---> Created $(RELEASE_ENT).tar.gz."

# make release-unix - Produces a binary release tarball containing teleport,
# tctl, and tsh.
.PHONY:
release-unix: clean full build-archive

# make full - produces a teleport binary with the web assets built-in
.PHONY:full
full: check-build-vars
	@echo "---> Attaching Enterprise web assets."
	$(MAKE) all WEBASSETS_TAG="webassets_embed webassets_ent"

.PHONY: check-build-vars
check-build-vars:
ifndef OS
	$(error OS is not set)
endif
ifndef ARCH
	$(error ARCH is not set)
endif
ifndef GITTAG
	$(error GITTAG is not set)
endif

#
# test runs teleport enterprise tests
#
.PHONY: test
test: test-go-all test-operator

#
# test-go-all runs all teleport enterprise go tests
#
test-go-all:
	$(MAKE) SUBJECT=./... test-go

#
# test-go runs all teleport enterprise go tests under the given
# subject directory. Use `SUBJECT=./...` to run all tests. If the
# subject is empty, no tests will be run.
#
test-go:
ifneq ("$(SUBJECT)", "")
	gotestsum -- $(SUBJECT) -race -shuffle on -tags "$(PIV_TEST_TAG) $(LIBFIDO2_TEST_TAG)" $(ADDFLAGS)
endif

# test-operator runs tests for the k8s operator with enterprise-only resource
# types that must be tested with an enterprise Teleport cluster.
include ../integrations/operator/envtest.mk
.PHONY: test-operator
test-operator: $(ENVTEST)
test-operator: export KUBEBUILDER_ASSETS=$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)
test-operator:
	gotestsum ./lib/operatortest -- -race

# Dockerized build: useful for making Linux binaries on OSX
.PHONY:docker-binaries
docker-binaries: clean
	make -C ../build.assets build-enterprise-binaries PIV=$(PIV)

.PHONY: build-image
build-image: OS=linux
build-image:
	cp ../build.assets/charts/Dockerfile $(BUILDDIR)/
	cd $(BUILDDIR) && docker build --no-cache . -t $(DOCKER_IMAGE):$(VERSION)$(PRODUCT_TAG)-$(ARCH) --target $(TARGET) \
		--build-arg DEB_PATH="./teleport-ent_$(VERSION)_$(ARCH).deb"

# Standard enterprise Docker image build. Always build the binaries themselves within docker (see
# the "docker" rule) to avoid dependencies on the host libc version.
.PHONY: image
image: OS=linux
image: TARBALL_PATH_SECTION:=-s "$(shell pwd)"
image: TARGET:=teleport
image: clean docker-binaries build-archive ent-deb build-image

.PHONY:docker-binaries-fips
docker-binaries-fips: clean
	@if [ -z ${VERSION} ]; then echo "VERSION is not set"; exit 1; fi
	make -C ../build.assets build-binaries-fips

# FIPS enterprise Docker image build. Always build the binaries themselves within docker (see
# the "docker" rule) to avoid dependencies on the host libc version.
.PHONY: image-fips
image-fips: OS=linux
image-fips: TARBALL_PATH_SECTION:=-s "$(shell pwd)"
image-fips: TARGET:=teleport-fips
image-fips: PRODUCT_TAG:=-fips
image-fips: clean docker-binaries-fips build-archive ent-deb build-image

RUNTIME_SECTION ?=
TARBALL_PATH_SECTION ?=

ifneq ("$(RUNTIME)", "")
	RUNTIME_SECTION := -r $(RUNTIME)
endif
ifneq ("$(ENT_TARBALL_PATH)", "")
	TARBALL_PATH_SECTION := -s $(ENT_TARBALL_PATH)
endif

# build .pkg
.PHONY: pkg
pkg: | $(RELEASE_DIR)
	mkdir -p $(BUILDDIR)/
	cp ../build.assets/build-package.sh ../build.assets/build-common.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	# arch and runtime are currently ignored on OS X
	# we pass them through for consistency - they will be dropped by the build script
	cd $(BUILDDIR) && ./build-package.sh -t ent -v $(VERSION) -p pkg -b $(TELEPORT_BUNDLEID) -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)
	cp $(BUILDDIR)/teleport-ent-*.pkg $(RELEASE_DIR)

# build .rpm
.PHONY: rpm
rpm:
	mkdir -p $(BUILDDIR)/
	cp ../build.assets/build-package.sh ../build.assets/build-common.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	cp -a ../build.assets/rpm-sign $(BUILDDIR)/
	cd $(BUILDDIR) && ./build-package.sh -t ent -v $(VERSION) -p rpm -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)

# build unsigned .rpm (for testing)
.PHONY: rpm-unsigned
rpm-unsigned:
	$(MAKE) UNSIGNED_RPM=true rpm

# build enterprise .deb only
.PHONY: ent-deb
ent-deb:
	mkdir -p $(BUILDDIR)/
	cp ../build.assets/build-package.sh ../build.assets/build-common.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	cd $(BUILDDIR) && ./build-package.sh -t ent -v $(VERSION) -p deb -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)

# Left here for compatiblity reasons
# build .deb
.PHONY: deb
deb: ent-deb

.PHONY: version
version:
	$(MAKE) -C .. version
