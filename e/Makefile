# This makefile usually runs as a sub-make by ../Makefile
# But running regular 'make' and 'make clean' works.
ifneq ("$(wildcard /bin/bash)","")
SHELL := /bin/bash -o pipefail
endif
BUILDDIR ?= build
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s $(LDFLAGS)' -trimpath
CGOFLAG ?= CGO_ENABLED=1
RELEASE_ENT = teleport-ent-$(GITTAG)-$(OS)-$(ARCH)-bin
BINARIES ?= $(BUILDDIR)/teleport $(BUILDDIR)/tctl $(BUILDDIR)/tsh $(BUILDDIR)/tbot
CLOUD_DOCKERFILE ?= ../build.assets/charts/Dockerfile

DOCKER_IMAGE_ENT_QUAY ?= quay.io/gravitational/teleport-ent
DOCKER_IMAGE_ENT_ECR ?= public.ecr.aws/gravitational/teleport-ent
DOCKER_IMAGE_ENT_STAGING ?= 146628656107.dkr.ecr.us-west-2.amazonaws.com/gravitational/teleport-ent

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
# ARM64 builds need to specify the correct C compiler
CGOFLAG = CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc
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
	@echo "---> Building docker image and deploying to cloud tenant."
	BASE_DOCKERFILE=$(CLOUD_DOCKERFILE) ./scripts/deploy-cloud.sh

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

# make release-unix - Produces a binary release tarball containing teleport,
# tctl, and tsh.
.PHONY:
release-unix: clean full
	@echo "---> Creating Enterprise release archive."
	cp ../README.md $(BUILDDIR)/
	cp ../CHANGELOG.md $(BUILDDIR)/
	cp ../build.assets/install $(BUILDDIR)/
	echo $(GITTAG) > $(BUILDDIR)/VERSION
	cp -rf $(BUILDDIR) teleport-ent
	cp -rf ../examples teleport-ent/
	tar $(TAR_FLAGS) -c teleport-ent | gzip -n > $(RELEASE_ENT).tar.gz
	rm -rf teleport-ent
	@echo "---> Created $(RELEASE_ENT).tar.gz."

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
	make -C ../build.assets build-enterprise-binaries

.PHONY: image
image: clean docker-binaries
	cp ../build.assets/charts/Dockerfile $(BUILDDIR)/
	cd $(BUILDDIR) && docker build --no-cache . -t $(DOCKER_IMAGE_ENT_QUAY):$(VERSION)

.PHONY:docker-binaries-fips
docker-binaries-fips: clean
	@if [ -z ${VERSION} ]; then echo "VERSION is not set"; exit 1; fi
	make -C ../build.assets build-binaries-fips

.PHONY: image-fips
image-fips: docker-binaries-fips
	@if [ -z ${VERSION} ]; then echo "VERSION is not set"; exit 1; fi
	cp ../build.assets/charts/Dockerfile-fips $(BUILDDIR)/Dockerfile
	cd $(BUILDDIR) && docker build --no-cache . -t $(DOCKER_IMAGE_ENT_QUAY):$(VERSION)-fips

.PHONY: publish
publish:
	docker push $(DOCKER_IMAGE_ENT_QUAY):$(VERSION)

.PHONY: publish-fips
publish-fips:
	docker push $(DOCKER_IMAGE_ENT_QUAY):$(VERSION)-fips

.PHONY: publish-ecr
publish-ecr:
	docker tag $(DOCKER_IMAGE_ENT_QUAY) $(DOCKER_IMAGE_ENT_ECR)
	docker push $(DOCKER_IMAGE_ENT_ECR):$(VERSION)

# Docker image build in CI.
# This is run to build and push Docker images to a private repository as part of the build process.
# When we are ready to make the images public after testing (i.e. when publishing a release), we pull these
# images down, retag them and push them up to the production repo so they're available for use.
# This job can be removed/consolidated after we switch over completely from using Jenkins to using Drone.
.PHONY: image-ci
image-ci: clean docker-binaries
	@if [ -z ${VERSION} ]; then echo "VERSION is not set"; exit 1; fi
	cp ../build.assets/charts/Dockerfile $(BUILDDIR)/
	cd $(BUILDDIR) && docker build --no-cache . -t $(DOCKER_IMAGE_ENT_STAGING):$(VERSION)

.PHONY: image-fips-ci
image-fips-ci: docker-binaries-fips
	@if [ -z ${VERSION} ]; then echo "VERSION is not set"; exit 1; fi
	cp ../build.assets/charts/Dockerfile-fips $(BUILDDIR)/Dockerfile
	cd $(BUILDDIR) && docker build --no-cache . -t $(DOCKER_IMAGE_ENT_STAGING):$(VERSION)-fips

.PHONY: publish-ci
publish-ci:
	@if DOCKER_CLI_EXPERIMENTAL=enabled docker manifest inspect $(DOCKER_IMAGE_ENT_STAGING):$(VERSION) 2>&1 >/dev/null; then\
		echo "$(DOCKER_IMAGE_ENT_STAGING):$(VERSION) already exists. ";     \
	else                                                                    \
		docker push $(DOCKER_IMAGE_ENT_STAGING):$(VERSION);                 \
	fi

.PHONY: publish-fips-ci
publish-fips-ci:
	@if DOCKER_CLI_EXPERIMENTAL=enabled docker manifest inspect $(DOCKER_IMAGE_ENT_STAGING):$(VERSION)-fips 2>&1 >/dev/null; then\
		echo "$(DOCKER_IMAGE_ENT_STAGING):$(VERSION) already exists. ";     \
	else                                                                    \
		docker push $(DOCKER_IMAGE_ENT_STAGING):$(VERSION)-fips;                 \
	fi

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

# build .deb
.PHONY: deb
deb:
	mkdir -p $(BUILDDIR)/
	cp ../build.assets/build-package.sh ../build.assets/build-common.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	cd $(BUILDDIR) && ./build-package.sh -t ent -v $(VERSION) -p deb -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)

.PHONY: ensure-webassets-e
ensure-webassets-e:
	$(MAKE) -s -C .. ensure-webassets-e
