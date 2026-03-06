# This makefile usually runs as a sub-make by ../Makefile
# But running regular 'make' and 'make clean' works.
ifneq ("$(wildcard /bin/bash)","")
SHELL := /bin/bash -o pipefail
endif
BUILDDIR ?= build
RELEASE_DIR ?= $(BUILDDIR)/artifacts
TEST_LOG_DIR ?= ${abspath ./test-logs}

TELEPORT_DEBUG ?= false
CGOFLAG ?= CGO_ENABLED=1
GITTAG ?= v$(VERSION)
DOCKER_IMAGE ?= teleport-ent
RELEASE_ENT = teleport-ent-$(GITTAG)-$(OS)-$(ARCH)-bin
BINS_default = teleport tctl tsh tbot fdpass-teleport teleport-update
BINS_darwin = teleport tctl tsh tbot fdpass-teleport
BINS = $(or $(BINS_$(OS)),$(BINS_default))
BINARIES ?= $(addprefix $(BUILDDIR)/,$(BINS))
KUSTOMIZE_NO_DYNAMIC_PLUGIN ?= kustomize_disable_go_plugin_support
KUBECTL_VERSION := $(shell go list -m -f '{{.Version}}' k8s.io/kubectl | sed 's/v0/v1/')
KUBECTL_SETVERSION := -X k8s.io/component-base/version.gitVersion=$(KUBECTL_VERSION)

# When TELEPORT_DEBUG is true, set flags so produce
# debugger-friendly builds.
ifeq ("$(TELEPORT_DEBUG)","true")
BUILDFLAGS ?= $(ADDFLAGS) -gcflags=all="-N -l" -ldflags '$(LDFLAGS) $(KUBECTL_SETVERSION)' -buildvcs=false
BUILDFLAGS_TBOT ?= $(ADDFLAGS) -gcflags=all="-N -l" -ldflags '$(LDFLAGS) $(KUBECTL_SETVERSION)' -buildvcs=false
else
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s $(LDFLAGS) $(KUBECTL_SETVERSION)' -trimpath -buildvcs=false
BUILDFLAGS_TBOT ?= $(ADDFLAGS) -ldflags '-w -s $(LDFLAGS) $(KUBECTL_SETVERSION)' -trimpath -buildvcs=false
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

# TODO(okraport) Temporary fallback during migration, remove once merged on both sides:
GOTESTSUM ?= go run gotest.tools/gotestsum@v1.13.0

# Check if rust and cargo are installed before compiling
CHECK_CARGO := $(shell cargo --version 2>/dev/null)
CHECK_RUST := $(shell rustc --version 2>/dev/null)

with_rdpclient := no
RDPCLIENT_ENT_MESSAGE := without-Windows-RDP-client

RUST_TARGET_ARCH ?= $(CARGO_TARGET_$(OS)_$(ARCH))

CARGO_TARGET_darwin_amd64 := x86_64-apple-darwin
CARGO_TARGET_darwin_arm64 := aarch64-apple-darwin
CARGO_TARGET_linux_arm := arm-unknown-linux-gnueabihf
CARGO_TARGET_linux_arm64 := aarch64-unknown-linux-gnu
CARGO_TARGET_linux_386 := i686-unknown-linux-gnu
CARGO_TARGET_linux_amd64 := x86_64-unknown-linux-gnu

CARGO_TARGET := --target=$(CARGO_TARGET_$(OS)_$(ARCH))

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

# Enable VNet daemon?
# With VNETDAEMON=yes, tsh uses a Launch Daemon to start VNet.
# This requires a signed and bundled tsh.
VNETDAEMON_MESSAGE := without-VNet-daemon
ifeq ("$(VNETDAEMON)", "yes")
VNETDAEMON_MESSAGE := with-VNet-daemon
VNETDAEMON_TAG := vnetdaemon
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
MESSAGES := $(PAM_ENT_MESSAGE) $(FIPS_ENT_MESSAGE) $(BPF_MESSAGE) $(RDPCLIENT_ENT_MESSAGE) $(LIBFIDO2_MESSAGE) $(TOUCHID_MESSAGE) $(PIV_MESSAGE) $(VNETDAEMON_MESSAGE)
WITH := $(subst -," ",$(call join-with,$(COMMA) ,$(subst with-,,$(filter with-%,$(MESSAGES)))))
WITHOUT := $(subst -," ",$(call join-with,$(COMMA) ,$(subst without-,,$(filter without-%,$(MESSAGES)))))
RELEASE_ENT_MESSAGE := "Building Enterprise with GOOS=$(OS) GOARCH=$(ARCH) with $(WITH) and without $(WITHOUT)."

# Set CGOFLAG and BUILDFLAGS as needed for the OS/ARCH.
ifeq ("$(OS)","linux")
ifeq (, $(filter $(ARCH), amd64 arm64))
# Link static version of libraries (bpf, pcsc) to reduce system dependencies.
CGOFLAG = CGO_ENABLED=1 CGO_CFLAGS="-I/usr/libbpf-${LIBBPF_VER}/include" CGO_LDFLAGS="-Wl,-Bstatic $(STATIC_LIBS) -Wl,-Bdynamic -Wl,--as-needed"
CGOFLAG_TSH = CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-Bstatic $(STATIC_LIBS_TSH) -Wl,-Bdynamic -Wl,--as-needed"

else ifeq ("$(ARCH)","arm")

# ARM builds need to specify the correct C compiler
CGOFLAG = CGO_ENABLED=1
# Add -debugtramp=2 to work around 24 bit CALL/JMP instruction offset.
BUILDFLAGS = $(ADDFLAGS) -ldflags '-extldflags "-Wl,--long-plt" -w -s -debugtramp=2 $(KUBECTL_SETVERSION)' -trimpath -buildvcs=false
BUILDFLAGS_TBOT = $(ADDFLAGS) -ldflags '-extldflags "-Wl,--long-plt" -w -s -debugtramp=2 $(KUBECTL_SETVERSION)' -trimpath -buildvcs=false

endif
endif

ifeq ("$(OS)","darwin")
# Set the minimum version for macOS builds for Go, Rust and Xcode builds.
# (as of Go 1.25 we require macOS 12)
MINIMUM_SUPPORTED_MACOS_VERSION = 12.0
MACOSX_VERSION_MIN_FLAG = -mmacosx-version-min=$(MINIMUM_SUPPORTED_MACOS_VERSION)

# Go
CGOFLAG = CGO_ENABLED=1 CGO_CFLAGS=$(MACOSX_VERSION_MIN_FLAG)

# Xcode and rust and Go linking
MACOSX_DEPLOYMENT_TARGET = $(MINIMUM_SUPPORTED_MACOS_VERSION)
export MACOSX_DEPLOYMENT_TARGET
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
	$(MAKE) -C .. e/$@ BUILDDIR=e/$(BUILDDIR)

.PHONY: $(BUILDDIR)/teleport
$(BUILDDIR)/teleport: ensure-webassets-e teleport-fips-check rdpclient
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -buildvcs=false -tags "webassets_embed webassets_ent $(PAM_TAG) $(FIPS_TAG) $(BPF_TAG) $(WEBASSETS_TAG) $(RDPCLIENT_TAG) $(PIV_BUILD_TAG) $(KUSTOMIZE_NO_DYNAMIC_PLUGIN)" -o $(BUILDDIR)/teleport $(BUILDFLAGS) ./tool/teleport

.PHONY: $(BUILDDIR)/tsh
$(BUILDDIR)/tsh:
	$(MAKE) -C .. e/$@ BUILDDIR=e/$(BUILDDIR)

.PHONY: $(BUILDDIR)/teleport-update
$(BUILDDIR)/teleport-update:
	$(MAKE) -C .. e/$@ BUILDDIR=e/$(BUILDDIR)

TBOT_DYNAMIC :=
ifneq ("$(FIPS)","")
# FIPS requires cgo and dynamic linking
	TBOT_DYNAMIC = yes
else ifeq ("$(CGO_ENABLED)", "1")
	TBOT_DYNAMIC = yes
endif

.PHONY: $(BUILDDIR)/tbot
$(BUILDDIR)/tbot: CGO_ENABLED ?= 0
$(BUILDDIR)/tbot:
ifeq ("$(TBOT_DYNAMIC)", "yes")
	GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=1 go build -tags "$(FIPS_TAG) $(KUSTOMIZE_NO_DYNAMIC_PLUGIN)" -o $(BUILDDIR)/tbot $(BUILDFLAGS_TBOT) ../tool/tbot
else
	GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 go build -tags "$(FIPS_TAG) $(KUSTOMIZE_NO_DYNAMIC_PLUGIN)" -o $(BUILDDIR)/tbot $(BUILDFLAGS_TBOT) ../tool/tbot
endif

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

.PHONY: $(BUILDDIR)/fdpass-teleport
$(BUILDDIR)/fdpass-teleport:
	cd ../tool/fdpass-teleport && cargo build --release --locked $(CARGO_TARGET)
	install ../tool/fdpass-teleport/target/$(RUST_TARGET_ARCH)/release/fdpass-teleport $(BUILDDIR)/

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

.PHONY: rdpclient
rdpclient:
ifeq ("$(with_rdpclient)", "yes")
	$(RDPCLIENT_ENV) \
		cargo build -p rdp-client $(if $(FIPS),--features=fips) --release --locked $(CARGO_TARGET)
endif

# If FIPS mode was requested, first compile an extra Teleport binary with symbols included and check that it contains BoringCrypto references.
.PHONY: teleport-fips-check
teleport-fips-check:
	@if [ "$(FIPS)" != "" ]; then \
		echo "FIPS mode requested, pre-compiling and checking binary for BoringCrypto symbols."; \
		GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG) $(WEBASSETS_TAG)" $(BUILDFLAGS) -o $(BUILDDIR)/teleport-unstripped -ldflags "" ./tool/teleport; \
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

# make delete-cloud - Removes a cloud tenant from the staging cluster.
.PHONY: delete-cloud
delete-cloud:
	./scripts/delete-cloud.sh

# make create-cloud - Provisions a new cloud tenant in the staging cluster.
.PHONY create-cloud:
create-cloud:
	./scripts/create-cloud.sh

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

SIGNED_BINARIES := $(BINARIES:%tsh=%tsh.app)
SIGNED_BINARIES := $(SIGNED_BINARIES:%tctl=%tctl.app)

.PHONY: release-darwin-unsigned
release-darwin-unsigned: RELEASE:=$(RELEASE)-unsigned
release-darwin-unsigned: full build-archive

.PHONY: release-darwin
ifneq ($(ARCH),universal)
release-darwin: release-darwin-unsigned
	$(NOTARIZE_BINARIES)
	$(MAKE) -C ../ tsh-app tctl-app BUILDDIR=e/$(BUILDDIR)
	$(MAKE) build-archive BINARIES="$(SIGNED_BINARIES)"
else
release-darwin: TARBINS := $(TARBINS:%tsh=%tsh.app)
release-darwin: TARBINS := $(TARBINS:%tctl=%tctl.app)
release-darwin: $(RELEASE_darwin_arm64) $(RELEASE_darwin_amd64)
	mkdir -p $(BUILDDIR_arm64) $(BUILDDIR_amd64)
	tar -C $(BUILDDIR_arm64) -xzf $(RELEASE_darwin_arm64) --strip-components=1 $(TARBINS)
	tar -C $(BUILDDIR_amd64) -xzf $(RELEASE_darwin_amd64) --strip-components=1 $(TARBINS)

	lipo -create -output $(BUILDDIR)/teleport $(BUILDDIR_arm64)/teleport $(BUILDDIR_amd64)/teleport
	lipo -create -output $(BUILDDIR)/tbot $(BUILDDIR_arm64)/tbot $(BUILDDIR_amd64)/tbot
	lipo -create -output $(BUILDDIR)/fdpass-teleport $(BUILDDIR_arm64)/fdpass-teleport $(BUILDDIR_amd64)/fdpass-teleport
	lipo -create -output $(BUILDDIR)/tsh \
		$(BUILDDIR_arm64)/tsh.app/Contents/MacOS/tsh \
		$(BUILDDIR_amd64)/tsh.app/Contents/MacOS/tsh
	lipo -create -output $(BUILDDIR)/tctl \
		$(BUILDDIR_arm64)/tctl.app/Contents/MacOS/tctl \
		$(BUILDDIR_amd64)/tctl.app/Contents/MacOS/tctl

	$(NOTARIZE_BINARIES)
	$(MAKE) -C ../ tsh-app tctl-app BUILDDIR=e/$(BUILDDIR)
	$(MAKE) ARCH=universal build-archive BINARIES="$(SIGNED_BINARIES)"
endif

#
# make build-archive - Packages the results of a build into a release tarball
#
.PHONY: build-archive
ifeq ("$(OS)","darwin")
build-archive: INSTALL_SCRIPT=../build.assets/macos/install
else
build-archive: INSTALL_SCRIPT=../build.assets/install
endif
build-archive: | $(RELEASE_DIR)
	@echo "---> Creating Enterprise release archive."
	mkdir teleport-ent
	cp -rf $(BINARIES) \
		../examples \
		"$(INSTALL_SCRIPT)" \
		../README.md \
		../CHANGELOG.md \
		teleport-ent/
	# add SELinux install script for Linux archives
	$(if $(filter linux,$(OS)), \
		cp ../assets/install-scripts/install-selinux.sh teleport-ent/ \
	)
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
# Install goda to lint testing symbols
#
.PHONY: ensure-goda
ensure-goda:
# Install goda if it's not already installed
 ifeq (, $(shell command -v goda))
	go install github.com/loov/goda@latest
endif

#
# Checks that testing symbols and the testify library is not included in binaries.
#
#
.PHONY: lint-test-symbols
lint-test-symbols: ensure-goda
	@testing_count=`goda tree "reach(github.com/gravitational/teleport/e/tool/...:all, testing)" | tee /dev/stderr | wc -l | tr -d ' '`; \
	if [ "$$testing_count" -gt 0 ]; then \
		echo ""; \
		echo "FAIL: \"testing\" is included in binaries"; \
	fi; \
	testify_count=`goda tree "reach(github.com/gravitational/teleport/e/tool/...:all, github.com/stretchr/testify/...)" | tee /dev/stderr | wc -l | tr -d ' '`; \
	if [ "$$testify_count" -gt 0 ]; then \
		echo ""; \
		echo "FAIL: \"github.com/stretchr/testify\" is included in binaries"; \
	fi; \
	if [ "$$testing_count" -gt 0 ] || [ "$$testify_count" -gt 0 ]; then \
		exit 1; \
	fi

$(TEST_LOG_DIR):
	mkdir $(TEST_LOG_DIR)

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
test-go: | $(TEST_LOG_DIR)
ifneq ("$(SUBJECT)", "")
	$(GOTESTSUM) --junitfile  $(TEST_LOG_DIR)/unit-tests.xml --jsonfile  $(TEST_LOG_DIR)/unit-tests.json -- $(SUBJECT) -race -shuffle on -tags "$(PIV_TEST_TAG) $(LIBFIDO2_TEST_TAG) $(TPMSIMULATOR_TEST_TAG) enablesynctest" $(ADDFLAGS)
endif

# test-operator runs tests for the k8s operator with enterprise-only resource
# types that must be tested with an enterprise Teleport cluster.
.PHONY: test-operator
test-operator: | $(TEST_LOG_DIR)
	$(GOTESTSUM) --junitfile  $(TEST_LOG_DIR)/unit-tests-operator.xml --jsonfile  $(TEST_LOG_DIR)/unit-tests-operator.json -- ./lib/operatortest -race -shuffle on

# test-crdb runs crdb backend tests.
.PHONY: test-crdb
test-crdb:
	make -C scripts/crdb test

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
pkg: TELEPORT_PKG_UNSIGNED = $(BUILDDIR)/teleport-ent-$(VERSION).unsigned.pkg
pkg: TELEPORT_PKG_SIGNED = $(RELEASE_DIR)/teleport-ent-$(VERSION).pkg
pkg: | $(RELEASE_DIR)
	mkdir -p $(BUILDDIR)/

	@echo Building tsh-$(VERSION).pkg
	../build.assets/build-pkg-app.sh -s build/artifacts -t ent -v $(VERSION) -b $(TSH_BUNDLEID) -a $(ARCH) $(TARBALL_PATH_SECTION)
	mv tsh*.pkg* $(BUILDDIR)/

	@echo Building tctl-$(VERSION).pkg
	../build.assets/build-pkg-app.sh -p tctl -t ent -v $(VERSION) -b $(TCTL_BUNDLEID) -a $(ARCH) $(TARBALL_PATH_SECTION)
	mv tctl*.pkg* $(BUILDDIR)/

	@echo Building teleport-bin-$(VERSION).pkg
	cp ../build.assets/build-package.sh ../build.assets/build-common.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	# runtime is currently ignored on OS X
	# we pass it through for consistency - it will be dropped by the build script
	cd $(BUILDDIR) && ./build-package.sh -t ent -v $(VERSION) -p pkg -b $(TELEPORT_BUNDLEID) -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)

	@echo Combining teleport-bin-$(VERSION).pkg and tsh-$(VERSION).pkg into teleport-ent-$(VERSION).pkg
	productbuild --package $(BUILDDIR)/tsh*.pkg --package $(BUILDDIR)/tctl*.pkg --package $(BUILDDIR)/teleport-ent-bin*.pkg $(TELEPORT_PKG_UNSIGNED)
	$(NOTARIZE_TELEPORT_PKG)

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

INTEGRATION_TEST_TIMEOUT ?= 10m
.PHONY: test-go-integration
test-go-integration: FLAGS ?= -race -shuffle on
test-go-integration: SUBJECT ?= ./tests/integration/...
test-go-integration: | $(TEST_LOG_DIR)
	@if [ -n "$(SUBJECT)" ]; then \
		$(CGOFLAG) go test -timeout=$(INTEGRATION_TEST_TIMEOUT) -json $(PACKAGES) $(SUBJECT) $(FLAGS) $(ADDFLAGS) \
			| $(GOTESTSUM) --junitfile $(TEST_LOG_DIR)/unit-tests-integration.xml --jsonfile $(TEST_LOG_DIR)/unit-tests-integration.json --raw-command -- cat; \
	fi

# All developers should run this upon cloning the repo. Unfortunately this
# can't be done automatically with git.
.PHONY: configure-git
configure-git:
	@./scripts/configure-git.sh

.PHONY: obfuscate-prompts
obfuscate-prompts:
	go run ./tooling/obfuscate-prompts
