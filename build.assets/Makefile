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
# Teleport version - some targets require this to be set
VERSION ?= $(shell egrep ^VERSION ../Makefile | cut -d= -f2)

ADDFLAGS ?=
BATSFLAGS :=
KUBECONFIG ?=
TEST_KUBE ?=

OS ?= linux

UID := $$(id -u)
GID := $$(id -g)
NOROOT := -u $(UID):$(GID)

include versions.mk
include arch.mk
# BUILDBOX_VERSION, BUILDBOX and BUILDBOX_variant variables are included in images.mk
include images.mk
include grpcbox.mk # Requires images.mk

DOCSBOX=ghcr.io/gravitational/docs

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

# The ARM64 build cannot be cross-compiled - the host architecture must be arm64
# for this build. Define a var that expands to an error if the build arch does
# not match the host arch.
#
# It should be expanded in a target's recipe to prevent it running if a cross-
# compile was requested.
#
# $(RUNTIME_ARCH) is the host architecture normalised to the Go arch.
# $(ARCH) is the target architecture we want to build for.
REQUIRE_HOST_ARCH = $(if $(filter-out $(ARCH),$(RUNTIME_ARCH)),$(error Cannot cross-compile $@ $(ARCH) on $(RUNTIME_ARCH)))

# This determines which make target we call in this repo's top level Makefile when
# make release in this Makefile is called. Currently this supports its default value
# (release) and release-unix-preserving-webassets. See the release-arm target for
# more details.
RELEASE_TARGET ?= release

#
# Build 'teleport' release inside a docker container
#
.PHONY:build
build: buildbox-centos7 webassets
	docker run $(DOCKERFLAGS) $(NOROOT) $(BUILDBOX_CENTOS7) \
		make -C $(SRCDIR) ADDFLAGS='$(ADDFLAGS)' PIV=$(PIV) release-unix-preserving-webassets

#
# Build 'teleport' release inside a docker container
#
.PHONY:build-binaries
build-binaries: buildbox-centos7 webassets
	docker run $(DOCKERFLAGS) $(NOROOT) $(BUILDBOX_CENTOS7) \
		make -C $(SRCDIR) ADDFLAGS='$(ADDFLAGS)' PIV=$(PIV) full

#
# Build 'teleport' Enterprise release inside a docker container
#
.PHONY:build-enterprise-binaries
build-enterprise-binaries: buildbox-centos7 webassets
	docker run $(DOCKERFLAGS) $(NOROOT) $(BUILDBOX_CENTOS7) \
		make -C $(SRCDIR)/e ADDFLAGS='$(ADDFLAGS)' VERSION=$(VERSION) GITTAG=v$(VERSION) PIV=$(PIV) clean full

#
# Build 'teleport' FIPS release inside a docker container
# This builds Enterprise binaries only.
#
.PHONY:build-binaries-fips
build-binaries-fips: buildbox-centos7-fips webassets
	docker run $(DOCKERFLAGS) $(NOROOT) $(BUILDBOX_CENTOS7_FIPS) \
		make -C $(SRCDIR)/e ADDFLAGS='$(ADDFLAGS)' VERSION=$(VERSION) GITTAG=v$(VERSION) PIV=$(PIV) FIPS=yes clean full

#
# Builds a Docker container which is used for building official Teleport binaries
# If running in CI and there is no image with the buildbox name:tag combination present locally,
# the image is pulled from the Docker repository. If this pull fails (i.e. when a new Go runtime is
# first used), the error is ignored and the buildbox is built using the Dockerfile.
# BUILDARCH is set explicitly, so it's set with and without BuildKit enabled.
#
.PHONY:buildbox
buildbox:
	docker buildx build \
		--build-arg UID=$(UID) \
		--build-arg GID=$(GID) \
		--build-arg BUILDARCH=$(RUNTIME_ARCH) \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		--build-arg GOLANGCI_LINT_VERSION=$(GOLANGCI_LINT_VERSION) \
		--build-arg RUST_VERSION=$(RUST_VERSION) \
		--build-arg WASM_PACK_VERSION=$(WASM_PACK_VERSION) \
		--build-arg NODE_VERSION=$(NODE_VERSION) \
		--build-arg LIBBPF_VERSION=$(LIBBPF_VERSION) \
		--build-arg BUF_VERSION=$(BUF_VERSION) \
		--build-arg GOGO_PROTO_TAG=$(GOGO_PROTO_TAG) \
		--build-arg NODE_GRPC_TOOLS_VERSION=$(NODE_GRPC_TOOLS_VERSION) \
		--build-arg NODE_PROTOC_TS_VERSION=$(NODE_PROTOC_TS_VERSION) \
		--build-arg PROTOC_VERSION=$(PROTOC_VERSION) \
		--cache-to type=inline \
		--cache-from $(BUILDBOX) \
		$(if $(PUSH),--push,--load) \
		--tag $(BUILDBOX) .

# Builds a Docker buildbox for FIPS
#
.PHONY:buildbox-fips
buildbox-fips: buildbox-centos7-fips

#
# Builds a Docker buildbox for CentOS 7 builds. It is built for amd64 and arm64
# but cannot be cross-compiled, so the host architecture must match the target
# architecture we are building.
#
# This buildbox uses the cached assets from the buildbox-centos7-assets container.
# build-centos7-assets can take a long time to complete, and thus it is not included
# as a dependency of this target in order to make local builds / development faster.
# It is called explicitly in CI before anything that depends on it is built, and when
# building locally it can be called manually if the assets need to be rebuilt.
#
.PHONY:buildbox-centos7
buildbox-centos7:
	$(REQUIRE_HOST_ARCH)
	docker buildx build \
		--target buildbox \
		--build-arg UID=$(UID) \
		--build-arg GID=$(GID) \
		--build-arg BUILDBOX_CENTOS7_ASSETS=$(BUILDBOX_CENTOS7_ASSETS) \
		--build-arg BUILDARCH=$(HOST_ARCH) \
		--build-arg TARGETARCH=$(RUNTIME_ARCH) \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		--build-arg RUST_VERSION=$(RUST_VERSION) \
		--build-arg WASM_PACK_VERSION=$(WASM_PACK_VERSION) \
		--build-arg DEVTOOLSET=$(DEVTOOLSET) \
		--build-arg LIBBPF_VERSION=$(LIBBPF_VERSION) \
		--build-arg LIBPCSCLITE_VERSION=$(LIBPCSCLITE_VERSION) \
		--cache-to type=inline \
		--cache-from $(BUILDBOX_CENTOS7) \
		$(if $(PUSH),--push,--load) \
		--tag $(BUILDBOX_CENTOS7) -f Dockerfile-centos7 .

#
# Builds a Docker buildbox for CentOS 7 FIPS builds. This uses the same
# Dockerfile as `buildbox-centos7` but with the `buildbox-fips` target which
# sets the appropriate environment variables and config files to build
# FIPS-compliant binaries. It also does a test build to ensure the built
# binaries are FIPS-compliant.
#
.PHONY:buildbox-centos7-fips
buildbox-centos7-fips:
	docker buildx build \
		--target buildbox-fips \
		--build-arg UID=$(UID) \
		--build-arg GID=$(GID) \
		--build-arg BUILDBOX_CENTOS7_ASSETS=$(BUILDBOX_CENTOS7_ASSETS) \
		--build-arg BUILDARCH=$(HOST_ARCH) \
		--build-arg TARGETARCH=$(RUNTIME_ARCH) \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		--build-arg RUST_VERSION=$(RUST_VERSION) \
		--build-arg WASM_PACK_VERSION=$(WASM_PACK_VERSION) \
		--build-arg DEVTOOLSET=$(DEVTOOLSET) \
		--build-arg LIBBPF_VERSION=$(LIBBPF_VERSION) \
		--build-arg LIBPCSCLITE_VERSION=$(LIBPCSCLITE_VERSION) \
		--cache-to type=inline \
		--cache-from $(BUILDBOX_CENTOS7_FIPS) \
		$(if $(PUSH),--push,--load) \
		--tag $(BUILDBOX_CENTOS7_FIPS) -f Dockerfile-centos7 .

#
# Builds a Docker buildbox for ARMv7/ARM64 builds
# ARM buildboxes use a regular Teleport buildbox as a base which already has a user
# with the correct UID and GID created, so those arguments are not needed here.
#
.PHONY:buildbox-arm
buildbox-arm:
	docker buildx build \
		--build-arg UID=$(UID) \
		--build-arg GID=$(GID) \
		--build-arg BUILDARCH=$(RUNTIME_ARCH) \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		--build-arg NODE_VERSION=$(NODE_VERSION) \
		--cache-to type=inline \
		--cache-from $(BUILDBOX_ARM) \
		$(if $(PUSH),--push,--load) \
		--tag $(BUILDBOX_ARM) -f Dockerfile-arm .

CONNECT_VERSION ?= $(VERSION)
ifeq ($(CONNECT_VERSION),)
CONNECT_VERSION := $(BUILDBOX_VERSION)-dev
endif

#
# Builds a Docker buildbox for builds that depend on Node.js.
# This can be used for building webassets as well as Teleport Connect.
#
.PHONY:buildbox-node
buildbox-node:
	docker buildx build \
		--build-arg BUILDARCH=$(RUNTIME_ARCH) \
		--build-arg UID=$(UID) \
		--build-arg GID=$(GID) \
		--build-arg NODE_VERSION=$(NODE_VERSION) \
		--build-arg RUST_VERSION=$(RUST_VERSION) \
		--build-arg WASM_PACK_VERSION=$(WASM_PACK_VERSION) \
		--cache-to type=inline \
		--cache-from $(BUILDBOX_NODE) \
		$(if $(PUSH),--push,--load) \
		--tag $(BUILDBOX_NODE) -f Dockerfile-node .

#
# Builds Teleport Connect inside the buildbox container.
#
.PHONY:teleterm
teleterm: buildbox-node
	docker run $(DOCKERFLAGS) $(NOROOT) $(BUILDBOX_NODE) \
		bash -c "cd $(SRCDIR) && export CONNECT_TSH_BIN_PATH=\$$PWD/../teleport/build/tsh && yarn install --frozen-lockfile && yarn build-term && yarn package-term -c.extraMetadata.version=$(CONNECT_VERSION)"

# Builds webassets inside Docker.
.PHONY:ui
ui: buildbox
	docker run -u $(UID):$(GID) -v "$$(pwd)/../":/teleport $(BUILDBOX) \
		bash -c "cd ../teleport && yarn install --frozen-lockfile && yarn build-ui"

# Builds webassets inside the buildbox-node container via the
# Makefile (instead of calling yarn directly as the ui target above does)
# as that computes a SHA to prevent unnecessary rebuilds. This is used by
# the CentOS 7 build that does not have a new enough version of node.js
# to build webassets. Enterprise webassets will only be built if the e
# submodule is present.
.PHONY: webassets
webassets: buildbox-node
	docker run $(DOCKERFLAGS) $(NOROOT) $(BUILDBOX_NODE) \
		/usr/bin/make ensure-webassets $(if $(wildcard ../e/Makefile),ensure-webassets-e)

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
		--env TELEPORT_XAUTH_TEST="yes" \
		$(DOCKERFLAGS) $(NOROOT) -t $(BUILDBOX) \
		/bin/bash -c \
		"ssh-agent > external.agent.tmp && source external.agent.tmp; \
		cd $(SRCDIR) && make TELEPORT_DEBUG=0 FLAGS='-cover -race' clean test"

.PHONY:test-root
test-root: buildbox
	docker run \
		--env TELEPORT_XAUTH_TEST="yes" \
		$(DOCKERFLAGS) -t $(BUILDBOX) \
		/bin/bash -c \
		"ssh-agent > external.agent.tmp && source external.agent.tmp; \
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
		--env TELEPORT_XAUTH_TEST="yes" \
		$(DOCKERFLAGS) $(NOROOT) -t $(BUILDBOX) \
		make -C $(SRCDIR) FLAGS='-cover' integration

.PHONY:integration-root
integration-root: buildbox
	docker run $(DOCKERFLAGS) -t $(BUILDBOX) \
		--env TELEPORT_XAUTH_TEST="yes" \
		/bin/bash -c "make -C $(SRCDIR) FLAGS='-cover' integration-root"

.PHONY:integration-kube
integration-kube: buildbox
	docker run $(DOCKERFLAGS) -t $(BUILDBOX) \
		/bin/bash -c "make -C $(SRCDIR) FLAGS='-cover' integration-kube"

.PHONY:e2e-aws
e2e-aws: buildbox
	docker run $(DOCKERFLAGS) -t $(BUILDBOX) \
		/bin/bash -c "make -C $(SRCDIR) FLAGS='-cover' e2e-aws"

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
enter/centos7: buildbox-centos7
	docker run $(DOCKERFLAGS) -ti $(NOROOT) \
		-e HOME=$(SRCDIR)/build.assets -w $(SRCDIR) $(BUILDBOX_CENTOS7) /bin/bash

#
# Starts a root shell inside the centos7 container
#
.PHONY:enter-root/centos7
enter-root/centos7: buildbox-centos7
	docker run $(DOCKERFLAGS) -ti \
		-e HOME=$(SRCDIR)/build.assets -w $(SRCDIR) $(BUILDBOX_CENTOS7) /bin/bash

#
# Starts shell inside the centos7-fips container
#
.PHONY:enter/centos7-fips
enter/centos7-fips: buildbox-centos7-fips
	docker run $(DOCKERFLAGS) -ti $(NOROOT) \
		-e HOME=$(SRCDIR)/build.assets -w $(SRCDIR) $(BUILDBOX_CENTOS7_FIPS) /bin/bash

#
# Starts a root shell inside the centos7-fips container
#
.PHONY:enter-root/centos7-fips
enter-root/centos7-fips: buildbox-centos7-fips
	docker run $(DOCKERFLAGS) -ti \
		-e HOME=$(SRCDIR)/build.assets -w $(SRCDIR) $(BUILDBOX_CENTOS7_FIPS) /bin/bash

#
# Starts a shell inside the grpcbox.
#
.PHONY: enter/grpcbox
enter/grpcbox: grpcbox
	$(GRPCBOX_RUN)

#
# Starts shell inside the node build container
#
.PHONY:enter/node
enter/node: buildbox-node
	docker run $(DOCKERFLAGS) -ti $(NOROOT) \
		-e HOME=$(SRCDIR)/build.assets -w $(SRCDIR) $(BUILDBOX_NODE) /bin/bash

.PHONY:enter/arm
enter/arm: buildbox-arm
	docker run $(DOCKERFLAGS) -ti $(NOROOT) \
		-e HOME=$(SRCDIR)/build.assets -w $(SRCDIR) $(BUILDBOX_ARM) /bin/bash

# #############################################################################
# Architecture and variant (fips) specific release targets
# These are aliases used to make build commands uniform.
# #############################################################################

# AMD64 builds are done on CentOS 7 build boxes for broader glibc compatibility.
.PHONY: release-amd64
release-amd64:
	$(MAKE) release-centos7 ARCH=amd64 FIDO2=yes PIV=yes

.PHONY: release-amd64-fips
release-amd64-fips:
	$(MAKE) release-centos7-fips ARCH=amd64 FIDO2=yes PIV=yes FIPS=yes

.PHONY: release-386
release-386:
	$(MAKE) release ARCH=386

.PHONY: release-arm64
release-arm64:
	$(MAKE) release-centos7 ARCH=arm64 FIDO2=yes PIV=yes

.PHONY: release-arm64-fips
release-arm64-fips:
	$(MAKE) release-centos7-fips ARCH=arm64 FIDO2=yes PIV=yes FIPS=yes

# We depend on webassets to pre-build them in the buildbox-node container
# so that we don't need to install node.js and rust on the buildbox-arm container.
# Therefore we set RELEASE_TARGET to release-unix-preserving-webassets.
.PHONY: release-arm
release-arm: webassets
	$(MAKE) release RELEASE_TARGET=release-unix-preserving-webassets ARCH=arm BUILDBOX=$(BUILDBOX_ARM)

# Compatibility targets for when we had separate Ubuntu and CentOS 7 targets for AMD64
# TODO(camscale): Remove these when drone no longer calls them.
.PHONY: release-amd64-centos7
release-amd64-centos7: release-amd64

.PHONY: release-amd64-centos7-fips
release-amd64-centos7-fips: release-amd64-fips

# #############################################################################
# Generic release targets
# Don't use these targets directly; call named Makefile targets such as `release-amd64`.
# #############################################################################

# Define buildbox makefile targets for building various buildboxes, parameterizing
# the `release` target`.
BUILDBOX_TARGET_$(BUILDBOX_ARM) = buildbox-arm

# Select the correct makefile target for building a buildbox based on $(BUILDBOX)
# with a default fallback of "buildbox".
BUILDBOX_TARGET = $(or $(BUILDBOX_TARGET_$(BUILDBOX)),buildbox)

#
# Create a Teleport package using the build container.
#
.PHONY:release
release: $(BUILDBOX_TARGET)
	@echo "Build Assets Release"
	docker run $(DOCKERFLAGS) $(NOROOT) $(BUILDBOX) \
		/usr/bin/make $(RELEASE_TARGET) -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(GOLANG_VERSION) FIDO2=$(FIDO2) PIV=$(PIV) REPRODUCIBLE=yes

#
# Create a Teleport FIPS package using the build container.
# This is a special case because it only builds and packages the Enterprise FIPS binaries, no OSS.
# CI should not use this target, it should use named Makefile targets like release-amd64-fips.
#
.PHONY:release-fips
release-fips: buildbox-centos7-fips webassets
	@if [ -z ${VERSION} ]; then echo "VERSION is not set"; exit 1; fi
	docker run $(DOCKERFLAGS) -i $(NOROOT) $(BUILDBOX_CENTOS7_FIPS) \
		/usr/bin/make -C e release -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(GOLANG_VERSION) FIDO2=$(FIDO2) PIV=$(PIV) FIPS=yes VERSION=$(VERSION) GITTAG=v$(VERSION) REPRODUCIBLE=yes

#
# Create a Teleport package for CentOS 7 using the build container.
# We depend on webassets to pre-build them in the buildbox-node container
# as the version of node.js on CentOS 7 is too old.
#
# CentOS 7 builds are for amd64 and arm64 and must be built (currently) on
# the same host architecture - i.e. no cross-compiling. CI (GitHub Actions)
# is set up to use an architecture-specific builder, but building locally
# for a foreign architecture cannot be done.
#
.PHONY:release-centos7
release-centos7: buildbox-centos7 webassets
	$(REQUIRE_HOST_ARCH)
	docker run $(DOCKERFLAGS) -i $(NOROOT) $(BUILDBOX_CENTOS7) \
		/usr/bin/scl enable $(DEVTOOLSET) 'make release-unix-preserving-webassets -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(GOLANG_VERSION) FIDO2=$(FIDO2) PIV=$(PIV) REPRODUCIBLE=no'

#
# Create a Teleport FIPS package for CentOS 7 using the build container.
# This only builds and packages enterprise FIPS binaries, no OSS.
# We depend on webassets to pre-build them in the buildbox-node container,
# as the version of node.js on CentOS 7 is too old. The enterprise Makefile
# does not clean the web assets on the `release` target so there is no need
# to call release-unix-preserving-webassets like we do for the non-fips release.
#
.PHONY:release-centos7-fips
release-centos7-fips: buildbox-centos7-fips webassets
	docker run $(DOCKERFLAGS) -i $(NOROOT) $(BUILDBOX_CENTOS7_FIPS) \
		/usr/bin/scl enable $(DEVTOOLSET) '/usr/bin/make -C e release -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(GOLANG_VERSION) FIDO2=$(FIDO2) PIV=$(PIV) FIPS=yes VERSION=$(VERSION) GITTAG=v$(VERSION) REPRODUCIBLE=no'

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
	docker run -i $(NOROOT) -v $$(pwd)/..:/src/content $(DOCSBOX) \
		/bin/sh -c "yarn markdown-lint-external-links"

#
# Print the Go version used to build Teleport.
#
.PHONY:print-go-version
print-go-version:
	@echo $(GOLANG_VERSION)

#
# Print the golangci-lint version used by Teleport.
#
.PHONY:print-golangci-lint-version
print-golangci-lint-version:
	@echo $(GOLANGCI_LINT_VERSION)

#
# Print the Buf version used by Teleport.
#
.PHONY:print-buf-version
print-buf-version:
	@echo $(BUF_VERSION)

#
# Print the Rust version used to build Teleport.
#
.PHONY:print-rust-version
print-rust-version:
	@echo $(RUST_VERSION)

#
# Print the wasm-pack version used to build Teleport.
#
.PHONY:print-wasm-pack-version
print-wasm-pack-version:
	@echo $(WASM_PACK_VERSION)

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
	docker buildx build \
		--build-arg BUILDARCH=$(HOST_ARCH) \
		--build-arg DEVTOOLSET=$(DEVTOOLSET) \
		--build-arg TARGETARCH=$(RUNTIME_ARCH) \
		--cache-to type=inline \
		--cache-from $(BUILDBOX_CENTOS7_ASSETS) \
		$(if $(PUSH),--push,--load) \
		--tag $(BUILDBOX_CENTOS7_ASSETS) \
		-f Dockerfile-centos7-assets .

#
# Generate event schema from proto files.
#
.PHONY:generate-eventschema
generate-eventschema:
	$(eval PROTOBUF_MOD_PATH := $(shell go mod download --json github.com/gogo/protobuf | awk -F: '/"Dir"/ { print $$2 }' | tr -d ' ",'))

	cd tooling && go build -o ./bin/protoc-gen-eventschema ./cmd/protoc-gen-eventschema/

	protoc \
		-I ../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=./tooling/bin/protoc-gen-eventschema \
		--eventschema_out="../gen/go/eventschema" \
		"teleport/legacy/types/events/events.proto";
