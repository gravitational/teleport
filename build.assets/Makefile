#
# This Makefile is used for producing official Teleport releases
#
ifneq ("$(wildcard /bin/bash)","")
SHELL := /bin/bash -o pipefail
endif
HOSTNAME=buildbox
SRCDIR=/go/src/github.com/gravitational/teleport
GOMODCACHE ?= /tmp/gomodcache
# TODO(hugoShaka) remove HELM_PLUGINS with teleport13 buildbox
DOCKERFLAGS := --rm=true -v "$$(pwd)/../":$(SRCDIR) -v /tmp:/tmp -w $(SRCDIR) -h $(HOSTNAME) -e GOMODCACHE=$(GOMODCACHE) -e HELM_PLUGINS=/home/ci/.local/share/helm/plugins-new

ADDFLAGS ?=
BATSFLAGS :=
NOROOT=-u $$(id -u):$$(id -g)
KUBECONFIG ?=
TEST_KUBE ?=

OS ?= linux
ARCH ?= amd64

GOLANG_VERSION ?= go1.20.5

NODE_VERSION ?= 16.18.1

# run lint-rust check locally before merging code after you bump this
RUST_VERSION ?= 1.68.0
LIBBPF_VERSION ?= 1.0.1
LIBPCSCLITE_VERSION ?= 1.9.9-teleport

# Protogen related versions.
BUF_VERSION ?= 1.23.1
# Keep in sync with api/proto/buf.yaml (and buf.lock)
GOGO_PROTO_TAG ?= v1.3.2
NODE_GRPC_TOOLS_VERSION ?= 1.12.4
NODE_PROTOC_TS_VERSION ?= 5.0.1
PROTOC_VER ?= 3.20.3

UID := $$(id -u)
GID := $$(id -g)

HOST_ARCH := $(shell uname -m)
RUNTIME_ARCH_x86_64 := amd64
# uname returns different value on Linux (aarch64) and MacOS (arm64).
RUNTIME_ARCH_arm64 := arm64
RUNTIME_ARCH_aarch64 := arm64
RUNTIME_ARCH := $(RUNTIME_ARCH_$(HOST_ARCH))

# BUILDBOX_VERSION, BUILDBOX and BUILDBOX_variant variables are included
include images.mk
include grpcbox.mk

# These variables are used to dynamically change the name of the buildbox Docker image used by the 'release'
# target. The other solution was to remove the 'buildbox' dependency from the 'release' target, but this would
# make it harder to run `make -C build.assets release` locally as the buildbox would not automatically be built.
BUILDBOX_NAME=$(BUILDBOX)
BUILDBOX_FIPS_NAME=$(BUILDBOX_FIPS)

DOCSBOX=public.ecr.aws/gravitational/docs

ifneq ("$(KUBECONFIG)","")
DOCKERFLAGS := $(DOCKERFLAGS) -v $(KUBECONFIG):/mnt/kube/config -e KUBECONFIG=/mnt/kube/config -e TEST_KUBE=$(TEST_KUBE)
endif

# conditionally force the use of UID/GID 1000:1000 if we're running in Drone or Github Actions (in which case CI env var will be set)
ifeq ("$(DRONE)","true")
CI := true
endif
ifeq ("$(CI)","true")
# The UID/GID of the runner user on ARC runners is 1001, not 1000
# This var is currently only set for ARC runners via https://github.com/gravitational/cloud-terraform/pull/2473
ifeq ("$(CI_SYSTEM)","ARC")
UID := 1001
GID := 1001
NOROOT := -u 1001:1001
else
UID := 1000
GID := 1000
NOROOT := -u 1000:1000
endif
# if running in CI and the GOCACHE environment variable is not set, set it to a sensible default
ifeq ("$(GOCACHE)",)
GOCACHE := /go/cache
endif
# pass external gocache path through to docker containers
DOCKERFLAGS := $(DOCKERFLAGS) -v $(GOCACHE):/go/cache -e GOCACHE=/go/cache
endif

ifeq ("$(RUNTIME_ARCH)","arm64")
	# devtoolset-11 is not available on CentOS 7 aarch64
	DEVTOOLSET ?= devtoolset-10
else
	DEVTOOLSET ?= devtoolset-11
endif

#
# Build 'teleport' release inside a docker container
#
.PHONY:build
build: buildbox-centos7
	docker run $(DOCKERFLAGS) $(NOROOT) $(BUILDBOX_CENTOS7) \
		make -C $(SRCDIR) ADDFLAGS='$(ADDFLAGS)' PIV=$(PIV) release

#
# Build 'teleport' release inside a docker container
#
.PHONY:build-binaries
build-binaries: buildbox-centos7
	docker run $(DOCKERFLAGS) $(NOROOT) $(BUILDBOX_CENTOS7) \
		make -C $(SRCDIR) ADDFLAGS='$(ADDFLAGS)' PIV=$(PIV) full

#
# Build 'teleport' Enterprise release inside a docker container
#
.PHONY:build-enterprise-binaries
build-enterprise-binaries: buildbox-centos7
	docker run $(DOCKERFLAGS) $(NOROOT) $(BUILDBOX_CENTOS7) \
		make -C $(SRCDIR)/e ADDFLAGS='$(ADDFLAGS)' VERSION=$(VERSION) GITTAG=v$(VERSION) PIV=$(PIV) clean full

#
# Build 'teleport' FIPS release inside a docker container
# This builds Enterprise binaries only.
#
.PHONY:build-binaries-fips
build-binaries-fips: buildbox-centos7-fips
	docker run $(DOCKERFLAGS) $(NOROOT) $(BUILDBOX_CENTOS7_FIPS) \
		make -C $(SRCDIR)/e ADDFLAGS='$(ADDFLAGS)' VERSION=$(VERSION) GITTAG=v$(VERSION) FIPS=yes clean full

#
# Builds a Docker container which is used for building official Teleport binaries
# If running in CI and there is no image with the buildbox name:tag combination present locally,
# the image is pulled from the Docker repository. If this pull fails (i.e. when a new Go runtime is
# first used), the error is ignored and the buildbox is built using the Dockerfile.
# BUILDARCH is set explicitly, so it's set with and without BuildKit enabled.
#
.PHONY:buildbox
buildbox:
	if [[ "$(BUILDBOX_NAME)" == "$(BUILDBOX)" ]]; then \
		if [[ $${DRONE} == "true" ]] && ! docker inspect --type=image $(BUILDBOX) 2>&1 >/dev/null; then docker pull $(BUILDBOX) || true; fi; \
		DOCKER_BUILDKIT=1 docker build --platform=linux/$(RUNTIME_ARCH) \
			--build-arg UID=$(UID) \
			--build-arg GID=$(GID) \
			--build-arg BUILDARCH=$(RUNTIME_ARCH) \
			--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
			--build-arg RUST_VERSION=$(RUST_VERSION) \
			--build-arg NODE_VERSION=$(NODE_VERSION) \
			--build-arg LIBBPF_VERSION=$(LIBBPF_VERSION) \
			--build-arg BUF_VERSION=$(BUF_VERSION) \
			--build-arg GOGO_PROTO_TAG=$(GOGO_PROTO_TAG) \
			--build-arg NODE_GRPC_TOOLS_VERSION=$(NODE_GRPC_TOOLS_VERSION) \
			--build-arg NODE_PROTOC_TS_VERSION=$(NODE_PROTOC_TS_VERSION) \
			--build-arg PROTOC_VER=$(PROTOC_VER) \
			--cache-from $(BUILDBOX) \
			--tag $(BUILDBOX) . ; \
	fi

# Builds a Docker buildbox for FIPS
#
.PHONY:buildbox-fips
buildbox-fips:
	if [[ "$(BUILDBOX_FIPS_NAME)" == "$(BUILDBOX_FIPS)" ]]; then \
		if [[ $${DRONE} == "true" ]] && ! docker inspect --type=image $(BUILDBOX_FIPS) 2>&1 >/dev/null; then docker pull $(BUILDBOX_FIPS) || true; fi; \
		docker build \
			--build-arg UID=$(UID) \
			--build-arg GID=$(GID) \
			--build-arg BUILDARCH=$(RUNTIME_ARCH) \
			--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
			--build-arg NODE_VERSION=$(NODE_VERSION) \
			--build-arg RUST_VERSION=$(RUST_VERSION) \
			--build-arg LIBBPF_VERSION=$(LIBBPF_VERSION) \
			--cache-from $(BUILDBOX_FIPS) \
			--tag $(BUILDBOX_FIPS) -f Dockerfile-fips . ; \
	fi

#
# Builds a Docker buildbox for CentOS 7 builds
#
.PHONY:buildbox-centos7
buildbox-centos7:
	@if [[ $${DRONE} == "true" ]] && ! docker inspect --type=image $(BUILDBOX_CENTOS7) 2>&1 >/dev/null; then docker pull $(BUILDBOX_CENTOS7) || true; fi;
	DOCKER_BUILDKIT=1 docker build \
		--build-arg UID=$(UID) \
		--build-arg GID=$(GID) \
		--build-arg BUILDARCH=$(RUNTIME_ARCH) \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		--build-arg NODE_VERSION=$(NODE_VERSION) \
		--build-arg RUST_VERSION=$(RUST_VERSION) \
		--build-arg DEVTOOLSET=$(DEVTOOLSET) \
		--build-arg LIBBPF_VERSION=$(LIBBPF_VERSION) \
		--build-arg LIBPCSCLITE_VERSION=$(LIBPCSCLITE_VERSION) \
		--cache-from $(BUILDBOX_CENTOS7) \
		--tag $(BUILDBOX_CENTOS7) -f Dockerfile-centos7 .

#
# Builds a Docker buildbox for CentOS 7 FIPS builds
#
.PHONY:buildbox-centos7-fips
buildbox-centos7-fips:
	@if [[ $${DRONE} == "true" ]] && ! docker inspect --type=image $(BUILDBOX_CENTOS7_FIPS) 2>&1 >/dev/null; then docker pull $(BUILDBOX_CENTOS7_FIPS) || true; fi;
	docker build \
		--build-arg UID=$(UID) \
		--build-arg GID=$(GID) \
		--build-arg BUILDARCH=$(RUNTIME_ARCH) \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		--build-arg NODE_VERSION=$(NODE_VERSION) \
		--build-arg RUST_VERSION=$(RUST_VERSION) \
		--build-arg LIBBPF_VERSION=$(LIBBPF_VERSION) \
		--cache-from $(BUILDBOX_CENTOS7_FIPS) \
		--tag $(BUILDBOX_CENTOS7_FIPS) -f Dockerfile-centos7-fips .

#
# Builds a Docker buildbox for ARMv7/ARM64 builds
# ARM buildboxes use a regular Teleport buildbox as a base which already has a user
# with the correct UID and GID created, so those arguments are not needed here.
#
.PHONY:buildbox-arm
buildbox-arm: buildbox
	@if [[ $${DRONE} == "true" ]] && ! docker inspect --type=image $(BUILDBOX_ARM) 2>&1 >/dev/null; then docker pull $(BUILDBOX_ARM) || true; fi;
	docker build \
		--build-arg BUILDBOX_VERSION=$(BUILDBOX_VERSION) \
		--cache-from $(BUILDBOX) \
		--cache-from $(BUILDBOX_ARM) \
		--tag $(BUILDBOX_ARM) -f Dockerfile-arm .

#
# Builds a Docker buildbox for ARMv7/ARM64 FIPS builds
# ARM buildboxes use a regular Teleport buildbox as a base which already has a user
# with the correct UID and GID created, so those arguments are not needed here.
#
.PHONY:buildbox-arm-fips
buildbox-arm-fips: buildbox-fips
	@if [[ $${DRONE} == "true" ]] && ! docker inspect --type=image $(BUILDBOX_ARM_FIPS) 2>&1 >/dev/null; then docker pull $(BUILDBOX_ARM_FIPS) || true; fi;
	docker build \
		--build-arg BUILDBOX_VERSION=$(BUILDBOX_VERSION) \
		--cache-from $(BUILDBOX_FIPS) \
		--cache-from $(BUILDBOX_ARM_FIPS) \
		--tag $(BUILDBOX_ARM_FIPS) -f Dockerfile-arm-fips .

CONNECT_VERSION ?= $(VERSION)
ifeq ($(CONNECT_VERSION),)
CONNECT_VERSION := $(BUILDBOX_VERSION)-dev
endif

#
# Builds a Docker buildbox for Linux Connect builds
#
.PHONY:buildbox-connect
buildbox-connect:
	if [[ $${DRONE} == "true" ]] && ! docker inspect --type=image $(BUILDBOX_CONNECT) 2>&1 >/dev/null; then docker pull $(BUILDBOX_CONNECT) || true; fi; \
	DOCKER_BUILDKIT=1 docker build \
		--build-arg NODE_VERSION=$(NODE_VERSION) \
		--cache-from $(BUILDBOX_CONNECT) \
		--tag $(BUILDBOX_CONNECT) -f Dockerfile-connect . ;

#
# Builds Teleport Connect inside the buildbox container.
#
.PHONY:teleterm
teleterm: buildbox-connect
	# Always run this image as user 1000, as the Node base image assumes that.
	docker run $(DOCKERFLAGS) -u 1000:1000 $(BUILDBOX_CONNECT) \
		bash -c "cd $(SRCDIR) && export CONNECT_TSH_BIN_PATH=\$$PWD/../teleport/build/tsh && yarn install --frozen-lockfile && yarn build-term && yarn package-term -c.extraMetadata.version=$(CONNECT_VERSION)"

# Builds webassets inside Docker.
.PHONY:ui
ui: buildbox
	docker run -u $(UID):$(GID) -v "$$(pwd)/../":/teleport $(BUILDBOX) \
		bash -c "cd ../teleport && yarn install --frozen-lockfile && yarn build-ui"

# grpc generates GRPC stubs from inside the buildbox
.PHONY: grpc
grpc: grpcbox
	$(GRPCBOX_RUN) make grpc/host

# protos-up-to-date checks if GRPC stubs are up to date from inside the buildbox
.PHONY: protos-up-to-date
protos-up-to-date: grpcbox
	$(GRPCBOX_RUN) make protos-up-to-date/host

# fix-imports runs GCI to sort and re-order Go imports in a deterministic way.
.PHONY: fix-imports
fix-imports: buildbox
	docker run \
		$(DOCKERFLAGS) -t $(BUILDBOX) \
		make -C /go/src/github.com/gravitational/teleport fix-imports/host

#
# Removes the docker image
#
.PHONY:clean
clean:
	docker image rm --force $(BUILDBOX)
	docker image rm --force $(DOCSBOX)
	GOMODCACHE=$(GOMODCACHE) go clean -modcache

#
# Runs tests inside a build container
#
.PHONY:test
test: buildbox
	docker run \
		--env TELEPORT_ETCD_TEST="yes" \
		--env TELEPORT_XAUTH_TEST="yes" \
		$(DOCKERFLAGS) $(NOROOT) -t $(BUILDBOX) \
		/bin/bash -c \
		"examples/etcd/start-etcd.sh & sleep 1; \
		type gcloud 2>&1 >/dev/null || exit 1; \
		gcloud -q beta emulators firestore start --host-port=localhost:8618 & sleep 1; \
		ssh-agent > external.agent.tmp && source external.agent.tmp; \
		cd $(SRCDIR) && make TELEPORT_DEBUG=0 FLAGS='-cover -race' clean test"

.PHONY:test-root
test-root: buildbox
	docker run \
		--env TELEPORT_ETCD_TEST="yes" \
		--env TELEPORT_XAUTH_TEST="yes" \
		$(DOCKERFLAGS) -t $(BUILDBOX) \
		/bin/bash -c \
		"examples/etcd/start-etcd.sh & sleep 1; \
		type gcloud 2>&1 >/dev/null || exit 1; \
		gcloud -q beta emulators firestore start --host-port=localhost:8618 & sleep 1; \
		ssh-agent > external.agent.tmp && source external.agent.tmp; \
		cd $(SRCDIR) && make TELEPORT_DEBUG=0 FLAGS='-cover -race' clean test-go-root"

.PHONY:test-sh
test-sh: buildbox
	docker run $(DOCKERFLAGS) $(NOROOT) -t $(BUILDBOX) \
		/bin/bash -c "make -C $(SRCDIR) BATSFLAGS=$(BATSFLAGS) test-sh"

.PHONY:test-helm
test-helm: buildbox
	docker run $(DOCKERFLAGS) $(NOROOT) -t $(BUILDBOX) \
		/bin/bash -c "make -C $(SRCDIR) test-helm"

.PHONY:test-helm-update-snapshots
test-helm-update-snapshots:
	docker run $(DOCKERFLAGS) $(NOROOT) -t $(BUILDBOX) \
		/bin/bash -c "make -C $(SRCDIR) test-helm-update-snapshots"

.PHONY:integration
integration: buildbox
	docker run \
		--env TELEPORT_ETCD_TEST="yes" \
		$(DOCKERFLAGS) $(NOROOT) -t $(BUILDBOX) \
		/bin/bash -c \
		"examples/etcd/start-etcd.sh & sleep 1; \
		make -C $(SRCDIR) FLAGS='-cover' integration"

.PHONY:integration-root
integration-root: buildbox
	docker run $(DOCKERFLAGS) -t $(BUILDBOX) \
		/bin/bash -c "make -C $(SRCDIR) FLAGS='-cover' integration-root"

#
# Runs linters on new changes inside a build container.
#
.PHONY:lint
lint: buildbox
	docker run $(DOCKERFLAGS) $(NOROOT) -t $(BUILDBOX) \
		/bin/bash -c "make -C $(SRCDIR) lint"

.PHONY:lint-helm
lint-helm: buildbox
	docker run $(DOCKERFLAGS) $(NOROOT) -t $(BUILDBOX) \
		/bin/bash -c "make -C $(SRCDIR) lint-helm"

#
# Starts shell inside the build container
#
.PHONY:enter
enter: buildbox
	docker run $(DOCKERFLAGS) -ti $(NOROOT) \
		-e HOME=$(SRCDIR)/build.assets -w $(SRCDIR) $(BUILDBOX) /bin/bash

#
# Starts a root shell inside the build container
#
.PHONY:enter-root
enter-root: buildbox
	docker run $(DOCKERFLAGS) -ti \
		-e HOME=$(SRCDIR)/build.assets -w $(SRCDIR) $(BUILDBOX) /bin/bash

#
# Starts shell inside the centos7 container
#
.PHONY:enter/centos7
enter/centos7: buildbox
	docker run $(DOCKERFLAGS) -ti $(NOROOT) \
		-e HOME=$(SRCDIR)/build.assets -w $(SRCDIR) $(BUILDBOX_CENTOS7) /bin/bash

#
# Starts a shell inside the grpcbox.
#
.PHONY: enter/grpcbox
enter/grpcbox: grpcbox
	$(GRPCBOX_RUN)

#
# Create a Teleport package using the build container.
# Don't use this target directly; call named Makefile targets like release-amd64.
#
.PHONY:release
release: buildbox
	@echo "Build Assets Release"
	docker run $(DOCKERFLAGS) $(NOROOT) $(BUILDBOX_NAME) \
		/usr/bin/make release -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(GOLANG_VERSION) FIDO2=$(FIDO2) PIV=$(PIV) REPRODUCIBLE=yes

# These are aliases used to make build commands uniform.
.PHONY: release-amd64
release-amd64:
	$(MAKE) release ARCH=amd64 FIDO2=yes

.PHONY: release-amd64-fips
release-amd64-fips:
	$(MAKE) release-fips ARCH=amd64 FIPS=yes BUILDBOX_FIPS_NAME=$(BUILDBOX_FIPS)

.PHONY: release-386
release-386:
	$(MAKE) release ARCH=386

.PHONY: release-arm
release-arm: buildbox-arm
	$(MAKE) release ARCH=arm BUILDBOX_NAME=$(BUILDBOX_ARM)

.PHONY: release-amd64-centos7
release-amd64-centos7: buildbox-centos7
	$(MAKE) release-centos7 ARCH=amd64 FIDO2=yes PIV=yes

.PHONY: release-amd64-centos7-fips
release-amd64-centos7-fips: buildbox-centos7-fips
	$(MAKE) release-centos7-fips ARCH=amd64 FIPS=yes

# #############################################################################
# Legacy Drone ARM build targets
# #############################################################################

.PHONY: release-arm64
release-arm64: buildbox-arm
	$(MAKE) release ARCH=arm64 BUILDBOX_NAME=$(BUILDBOX_ARM)

# #############################################################################
# GHA release build targets
# #############################################################################

#
# The `release-oss` and `release-enterprise` make targets are for building OSS
# and Enterprise releases from GitHub actions. These make targets execute in a
# container based on a caller-supplied `BUILDBOX` image.
#
# Unlike the other release targets, these release targets do not attempt to
# automatically construct the buildbox image at build time - the build box
# image *must* be pre-built and the tag supplied via `BUILDBOX=...` on the
# make command line
#
# Separating the construction of build environment from the actual build itself
# allows us to use the appropriate GHA actions to pre-build the buildbox
# image, so that the image construction works nicely with GHA.
#

define RELEASE_RECIPE
		docker run \
			--rm \
			--platform linux/$(ARCH) \
			--memory-swap -1 \
			-h buildbox \
			-e GOMODCACHE=$(GOMODCACHE) \
			-e GOCACHE=$(GOCACHE) \
			-v "/tmp:/tmp" \
			-v "$(WORKSPACE):/go/teleport" \
			-w /go/teleport \
			-u $(UID):$(GID) \
			$(BUILDBOX) \
		make $(1) ARCH=$(ARCH) ADDFLAGS="$(ADDFLAGS)" OS=$(OS) RUNTIME=$(GOLANG_VERSION) FIDO2=$(FIDO2) PIV=$(PIV) REPRODUCIBLE=no
endef

.PHONY: release-oss
release-oss:
	$(call RELEASE_RECIPE,full build-archive)

.PHONY: release-enterprise
release-enterprise:
	$(call RELEASE_RECIPE,release-unix-only)

# #############################################################################

#
# Create a Teleport FIPS package using the build container.
# This is a special case because it only builds and packages the Enterprise FIPS binaries, no OSS.
# CI should not use this target, it should use named Makefile targets like release-amd64-fips.
#
.PHONY:release-fips
release-fips: buildbox-fips
	@if [ -z ${VERSION} ]; then echo "VERSION is not set"; exit 1; fi
	docker run $(DOCKERFLAGS) -i $(NOROOT) $(BUILDBOX_FIPS_NAME) \
		/usr/bin/make -C e release -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(GOLANG_VERSION) FIPS=yes VERSION=$(VERSION) GITTAG=v$(VERSION) REPRODUCIBLE=yes

#
# Create a Teleport package for CentOS 7 using the build container.
#
.PHONY:release-centos7
release-centos7: buildbox-centos7
	docker run $(DOCKERFLAGS) -i $(NOROOT) $(BUILDBOX_CENTOS7) \
		/usr/bin/scl enable $(DEVTOOLSET) 'make release -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(GOLANG_VERSION) FIDO2=$(FIDO2) PIV=$(PIV) REPRODUCIBLE=no'

#
# Create a Teleport FIPS package for CentOS 7 using the build container.
# This only builds and packages enterprise FIPS binaries, no OSS.
#
.PHONY:release-centos7-fips
release-centos7-fips:
	docker run $(DOCKERFLAGS) -i $(NOROOT) $(BUILDBOX_CENTOS7_FIPS) \
		/usr/bin/scl enable $(DEVTOOLSET) '/usr/bin/make -C e release -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(GOLANG_VERSION) FIPS=yes VERSION=$(VERSION) GITTAG=v$(VERSION) REPRODUCIBLE=no'

#
# Create a Windows Teleport package using the build container.
#
.PHONY:release-windows
release-windows: buildbox
	docker run $(DOCKERFLAGS) -i $(NOROOT) $(BUILDBOX) \
		/usr/bin/make release -e ADDFLAGS="$(ADDFLAGS)" OS=windows RUNTIME=$(GOLANG_VERSION) REPRODUCIBLE=yes PIV=yes

#
# Create an unsigned Windows Teleport package using the build container.
#
.PHONY:release-windows-unsigned
release-windows-unsigned: buildbox
	docker run $(DOCKERFLAGS) -i $(NOROOT) $(BUILDBOX) \
		/usr/bin/make release-windows-unsigned -e ADDFLAGS="$(ADDFLAGS)" OS=windows RUNTIME=$(GOLANG_VERSION) REPRODUCIBLE=yes PIV=yes

#
# Run docs tester to detect problems.
#
.PHONY:docsbox
docsbox:
	if ! docker inspect --type=image $(DOCSBOX) 2>&1 >/dev/null; then docker pull $(DOCSBOX) || true; fi

.PHONY:test-docs
test-docs: docsbox
	docker run --platform=linux/amd64 -i $(NOROOT) -v $$(pwd)/..:/src/content $(DOCSBOX) \
		/bin/sh -c "yarn markdown-lint-external-links"

#
# Print the Go version used to build Teleport.
#
.PHONY:print-go-version
print-go-version:
	@echo $(GOLANG_VERSION)

#
# Print the Rust version used to build Teleport.
#
.PHONY:print-rust-version
print-rust-version:
	@echo $(RUST_VERSION)

#
# Print the Node version used to build Teleport Connect.
#
.PHONY:print-node-version
print-node-version:
	@echo $(NODE_VERSION)

#
# Print the buildbox version used to build Teleport.
#
.PHONY:print-buildbox-version
print-buildbox-version:
	@echo $(BUILDBOX_VERSION)

#
# Build CentOS 7 assets - clang.
#
.PHONY:build-centos7-assets
build-centos7-assets:
	docker build \
		--build-arg DEVTOOLSET=$(DEVTOOLSET) \
		--tag $(BUILDBOX_CENTOS7_ASSETS)-$(RUNTIME_ARCH) \
		-f Dockerfile-centos7-assets .
