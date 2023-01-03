# This makefile usually runs as a sub-make by ../Makefile
# But running regular 'make' and 'make clean' works.
ifneq ("$(wildcard /bin/bash)","")
SHELL := /bin/bash -o pipefail
endif
BUILDDIR ?= build
TELEPORT_DEBUG ?= false
CGOFLAG ?= CGO_ENABLED=1
GITTAG ?= v$(VERSION)
DOCKER_IMAGE ?= teleport-ent
RELEASE_ENT = teleport-ent-$(GITTAG)-$(OS)-$(ARCH)-bin
BINARIES ?= $(BUILDDIR)/teleport $(BUILDDIR)/tctl $(BUILDDIR)/tsh $(BUILDDIR)/tbot
CLOUD_DOCKERFILE ?= ../build.assets/charts/Dockerfile

# When TELEPORT_DEBUG is true, set flags so produce
# debugger-friendly builds.
ifeq ("$(TELEPORT_DEBUG)","true")
BUILDFLAGS ?= $(ADDFLAGS) -gcflags=all="-N -l"
else
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s $(LDFLAGS)' -trimpath
endif

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

FIPS_ENT_MESSAGE := "without-FIPS-support"
# FIPS support must be requested at build time.
ifneq ("$(FIPS)","")
FIPS_TAG := fips
FIPS_ENT_MESSAGE := "with-FIPS-support"
RELEASE_ENT = teleport-ent-$(GITTAG)-$(OS)-$(ARCH)-fips-bin
# See the comment in tool/modules/fips.go.hide for explanation.
$(shell ln -sf fips.go.hide tool/modules/fips.go)
else
# See the comment in tool/modules/fips.go.hide for explanation.
$(shell rm -f tool/modules/fips.go)
endif

PAM_ENT_MESSAGE := "without-PAM-support"
# PAM support will only be built into Teleport if headers exist at build time.
ifneq ("$(wildcard /usr/include/security/pam_appl.h)","")
PAM_TAG = pam
PAM_ENT_MESSAGE := "with-PAM-support"
endif

BPF_ENT_MESSAGE := "without-BPF-support"
# We don't compile BPF for anything except regular non-FIPS linux/amd64 for now, as other builds
# have compilation issues that require fixing.
with_bpf := no
ifeq ("$(OS)","linux")
ifeq ("$(ARCH)","amd64")
ifneq ("$(wildcard /usr/include/bpf/libbpf.h)","")
with_bpf := yes
BPF_TAG := bpf
BPF_ENT_MESSAGE := "with-BPF-support"
INCLUDES :=

STATIC_LIBS += -lbpf -lelf -lz
endif
endif
endif

# Check if rust and cargo are installed before compiling
CHECK_CARGO := $(shell cargo --version 2>/dev/null)
CHECK_RUST := $(shell rustc --version 2>/dev/null)

with_rdpclient := no
RDPCLIENT_ENT_MESSAGE := "without-Windows-RDP-client"

CARGO_TARGET_darwin_amd64 := x86_64-apple-darwin
CARGO_TARGET_darwin_arm64 := aarch64-apple-darwin
CARGO_TARGET_linux_arm := arm-unknown-linux-gnueabihf
CARGO_TARGET_linux_arm64 := aarch64-unknown-linux-gnu
CARGO_TARGET_linux_386 := i686-unknown-linux-gnu
CARGO_TARGET_linux_amd64 := x86_64-unknown-linux-gnu

CARGO_TARGET := --target=${CARGO_TARGET_${OS}_${ARCH}}

ifneq ($(CHECK_RUST),)
ifneq ($(CHECK_CARGO),)

# Do not build RDP client on ARM or 386.
ifneq ("$(ARCH)","arm")
ifneq ("$(ARCH)","386")
with_rdpclient := yes
RDPCLIENT_ENT_MESSAGE := "with-Windows-RDP-client"
RDPCLIENT_TAG := desktop_access_rdp
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
MESSAGES := $(PAM_ENT_MESSAGE) $(FIPS_ENT_MESSAGE) $(BPF_ENT_MESSAGE) $(RDPCLIENT_ENT_MESSAGE) $(LIBFIDO2_MESSAGE) $(TOUCHID_MESSAGE) $(PIV_MESSAGE)
WITH := $(subst -," ",$(call join-with,$(COMMA) ,$(subst with-,,$(filter with-%,$(MESSAGES)))))
WITHOUT := $(subst -," ",$(call join-with,$(COMMA) ,$(subst without-,,$(filter without-%,$(MESSAGES)))))
RELEASE_ENT_MESSAGE := "Building Enterprise with GOOS=$(OS) GOARCH=$(ARCH) and with $(WITH) and without $(WITHOUT)."

# Is this build targeting the same OS & architecture it is being compiled on, or
# will it require cross-compilation? We need to know this (especially for ARM) so we 
# can set the cross-compiler path correctly. 
IS_CROSS_BUILD = $(if $(filter-out $(ARCH), $(shell go env GOARCH)),yes)

# Set CGOFLAG and BUILDFLAGS as needed for the OS/ARCH.
ifeq ("$(OS)","linux")
ifeq ("$(ARCH)","amd64")
# Link static version of libraries (bpf, pcsc) to reduce system dependencies.
CGOFLAG = CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-Bstatic $(STATIC_LIBS) -Wl,-Bdynamic"
CGOFLAG_TSH = CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-Bstatic $(STATIC_LIBS_TSH) -Wl,-Bdynamic"
else ifeq ("$(ARCH)","arm")
# ARM builds need to specify the correct C compiler
CGOFLAG = CGO_ENABLED=1 CC=arm-linux-gnueabihf-gcc
else ifeq ("$(ARCH)","arm64")

# ARM64 requires CGO but does not need to do any special linkage due to its reduced 
# featureset. Also, if we 're not guaranteed to be building natively on an arm64 system
# then we'll need to configure the cross compiler.

CGOFLAG = CGO_ENABLED=1 $(if $(IS_CROSS_BUILD),CC=aarch64-linux-gnu-gcc)

endif
endif

CGOFLAG_TSH ?= $(CGOFLAG)

# All build Enterprise version of "teleport" and "tctl" binaries and copy
# over tsh from OSS.
.PHONY: all
all:
	@echo "---> $(RELEASE_ENT_MESSAGE)"
	$(MAKE) $(BINARIES)

# By making these 3 targets below (tsh, tctl and teleport) PHONY we are solving
# several problems:
# * Build will rely on go build internal caching https://golang.org/doc/go1.10 at all times
# * Manual change detection was broken on a large dependency tree
# If you are considering changing this behavior, please consult with dev team first
.PHONY: $(BUILDDIR)/tctl
$(BUILDDIR)/tctl:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -buildvcs=false -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG) $(PIV_BUILD_TAG)" -o $(BUILDDIR)/tctl $(BUILDFLAGS) ./tool/tctl

.PHONY: $(BUILDDIR)/teleport
$(BUILDDIR)/teleport: bpf-bytecode-e ensure-webassets-e teleport-fips-check rdpclient
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -buildvcs=false -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG) $(WEBASSETS_TAG) $(RDPCLIENT_TAG) $(PIV_BUILD_TAG)" -o $(BUILDDIR)/teleport $(BUILDFLAGS) ./tool/teleport

.PHONY: $(BUILDDIR)/tsh
$(BUILDDIR)/tsh:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG_TSH) go build -buildvcs=false -tags "$(PAM_TAG) $(FIPS_TAG) $(LIBFIDO2_BUILD_TAG) $(TOUCHID_TAG) $(PIV_BUILD_TAG)" -o $(BUILDDIR)/tsh $(BUILDFLAGS) ../tool/tsh

.PHONY: $(BUILDDIR)/tbot
$(BUILDDIR)/tbot:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -buildvcs=false -tags "$(FIPS_TAG)" -o $(BUILDDIR)/tbot $(BUILDFLAGS) ../tool/tbot

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

ifeq ("$(with_rdpclient)", "yes")
.PHONY: rdpclient
rdpclient:
	cargo build -p rdp-client --release $(CARGO_TARGET)
	if [[ ! $$(cbindgen --version 2>/dev/null) ]]; then cargo install cbindgen; fi
	cbindgen --quiet --crate rdp-client --output ../lib/srv/desktop/rdp/rdpclient/librdprs.h --lang c ../lib/srv/desktop/rdp/rdpclient/
else
.PHONY: rdpclient
rdpclient:
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
			echo "-- BoringCrypto is currently only supported for linux/amd64 as per https://go.googlesource.com/go/+/dev.boringcrypto/misc/boring#caveat"; \
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
deploy-cloud: ensure-webassets-e
	@echo "---> Building teleport binaries for deployment to cloud tenant."
	GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -buildvcs=false -tags "webassets_embed webassets_ent" -o $(BUILDDIR)/teleport $(BUILDFLAGS) ./tool/teleport
	@if [ "$(CLOUD_DOCKERFILE)" != "-" ]; then \
		echo "---> Building tbot, tctl and tsh."; \
		GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -buildvcs=false -o $(BUILDDIR)/tctl $(BUILDFLAGS) ./tool/tctl; \
		GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -buildvcs=false -o $(BUILDDIR)/tsh $(BUILDFLAGS) ../tool/tsh; \
		GOOS=linux GOARCH=amd64 CGO_ENABLED=1 go build -buildvcs=false -o $(BUILDDIR)/tbot $(BUILDFLAGS) ../tool/tbot; \
	else \
		echo "---> Skipping tbot, tctl and tsh."; \
	fi
	$(MAKE) build-archive ent-deb OS=linux GITTAG=v99.99.99-DEV VERSION=99.99.99-DEV ARCH=amd64 TARBALL_PATH_SECTION="-s \"$(shell pwd)\"" TARGET=teleport
	@echo "---> Building docker image and deploying to cloud tenant."
	BASE_DOCKERFILE=$(CLOUD_DOCKERFILE) DOCKERFILE_TARGET="teleport" DEB_PATH="./teleport-ent_99.99.99-DEV_amd64.deb" ./scripts/deploy-cloud.sh

# make deploy-cloud-login - Ensures access to all services required to deploy
# an image to a cloud tenant in the staging cluster.
.PHONY: deploy-cloud-login
deploy-cloud-login:
	./scripts/deploy-cloud-login.sh

# make release - Produces a binary release tarball.
.PHONY:
export
release:
	@echo
	@echo "---> $(RELEASE_ENT_MESSAGE)"
	$(MAKE) --no-print-directory release-unix

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

#
# make build-archive - Packages the results of a build into a release tarball
#
.PHONY: build-archive
build-archive:
	@echo "---> Creating Enterprise release archive."
	mkdir -p $(BUILDDIR)/
	cp ../README.md $(BUILDDIR)/
	cp ../CHANGELOG.md $(BUILDDIR)/
	cp ../build.assets/install $(BUILDDIR)/
	echo $(GITTAG) > $(BUILDDIR)/VERSION
	cp -rf $(BUILDDIR) teleport-ent
	cp -rf ../examples teleport-ent/
	tar $(TAR_FLAGS) -c teleport-ent | gzip -n > $(RELEASE_ENT).tar.gz
	rm -rf teleport-ent
	@echo "---> Created $(RELEASE_ENT).tar.gz."

# make release-unix - Produces a binary release tarball containing teleport,
# tctl, and tsh.
.PHONY:
release-unix: clean full build-archive

# make full - produces a teleport binary with the web assets built-in
.PHONY:full
full: check-build-vars ensure-webassets-e
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
test:
	go test -v ./...

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
pkg:
	mkdir -p $(BUILDDIR)/
	cp ../build.assets/build-package.sh ../build.assets/build-common.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	# arch and runtime are currently ignored on OS X
	# we pass them through for consistency - they will be dropped by the build script
	cd $(BUILDDIR) && ./build-package.sh -t ent -v $(VERSION) -p pkg -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)

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

.PHONY: ensure-webassets-e
ensure-webassets-e:
	$(MAKE) -s -C .. ensure-webassets-e
