# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 0.0.1

# Image URL to use all building/pushing image targets
IMG ?= teleport-operator:latest

# Include build.assets/versions.mk to get the latest same versions of tooling
# across all builds.
include ../../build.assets/versions.mk

# Setting SHELL to bash allows bash commands to be executed by recipes.
# This is a requirement for 'setup-envtest.sh' in the test target.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# include BUILDBOX_VERSION, BUILDBOX and BUILDBOX_variant variables
include ../../build.assets/images.mk

# include setup-envtest binary
include ./envtest.mk

# Configure which compiler and buildbox to use
OS ?= $(shell go env GOOS)
# ARCH is the architecture of the current machine. For cross-compilation, use
# TARGETARCH instead.
ARCH ?= $(shell go env GOARCH)
# TARGETARCH is the architecture to build for, which may be different from the
# architecture of the current machine. Accepted values are: amd64, arm, arm64 and 386.
# For native builds ARCH == TARGETARCH.
TARGETARCH ?= $(ARCH)

ifeq ("$(OS)","linux")
ifeq ("$(TARGETARCH)","amd64")
COMPILER ?= x86_64-linux-gnu-gcc
PLATFORM_BUILDBOX ?= $(BUILDBOX)
else ifeq ("$(TARGETARCH)","386")
COMPILER ?= i686-linux-gnu-gcc
PLATFORM_BUILDBOX ?= $(BUILDBOX)
else ifeq ("$(TARGETARCH)","arm")
COMPILER ?= arm-linux-gnueabihf-gcc
PLATFORM_BUILDBOX ?= $(BUILDBOX_ARM)
else ifeq ("$(TARGETARCH)","arm64")
COMPILER ?= aarch64-linux-gnu-gcc
PLATFORM_BUILDBOX ?= $(BUILDBOX_ARM)
endif
endif

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development


.PHONY: crd ## Single command to generate anything CRD-related (manifests and docs)
crd: crd-docs crd-manifests

.PHONY: generate
generate: generate-types crd ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.

generate-types: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

crdgenfiles := $(wildcard crdgen/*.go)

.PHONY: crdgen ## Builds both crd and crd documentation protoc plugins.
crdgen: crdgen/build/protoc-gen-crd crdgen/build/protoc-gen-crd-docs

crdgen/build/%: crdgen/cmd/%/main.go crdgen/cmd/%/debug.go $(crdgenfiles)
	mkdir -p crdgen/build
	go build -o $@ github.com/gravitational/teleport/integrations/operator/crdgen/cmd/$*

# List of protos used for CRD generation.
# If you are adding a new resource, you will want to add your proto file here.
PROTOS = \
	teleport/loginrule/v1/loginrule.proto \
	teleport/accesslist/v1/accesslist.proto \
	teleport/legacy/types/types.proto \
	teleport/machineid/v1/bot.proto \
	teleport/workloadidentity/v1/resource.proto \
	teleport/autoupdate/v1/autoupdate.proto

.PHONY: crd-manifests
crd-manifests: crdgen ## Generates CRD manifests using crdgen.
	@echo "## Generating CRD manifests"
# The wrappers.proto file needed for this generator exist only inside the go mod cache,
# so we retrieve the file path for the cached proto files with go mod tools.
	$(eval PROTOBUF_MOD_PATH := $(shell go mod download --json github.com/gogo/protobuf | awk -F: '/"Dir"/ { print $$2 }' | tr -d ' ",'))

	for proto in $(PROTOS); do \
		protoc \
			-I=../../api/proto \
			-I=$(PROTOBUF_MOD_PATH) \
			--plugin=./crdgen/build/protoc-gen-crd \
			--crd_out=:./config/crd/bases \
			"$${proto}"; \
	done

	rm -r ../../examples/chart/teleport-cluster/charts/teleport-operator/operator-crds/
	cp -r ./config/crd/bases/ ../../examples/chart/teleport-cluster/charts/teleport-operator/operator-crds

.PHONY: crd-docs
crd-docs: crdgen/build/protoc-gen-crd-docs ## Generate CRD reference docs.
	@echo "## Generating CRD documentation"
# The wrappers.proto file needed for this generator exist only inside the go mod cache,
# so we retrieve the file path for the cached proto files with go mod tools.
	$(eval PROTOBUF_MOD_PATH := $(shell go mod download --json github.com/gogo/protobuf | awk -F: '/"Dir"/ { print $$2 }' | tr -d ' ",'))

	for proto in $(PROTOS); do \
		protoc \
			-I=../../api/proto \
			-I=$(PROTOBUF_MOD_PATH) \
			--plugin=./crdgen/build/protoc-gen-crd-docs \
			--crd-docs_out=../../docs/pages/reference/operator-resources/ \
			"$${proto}"; \
	done

# crd-manifests-diff renders CRDs using crdgen in a temporary directory, then compares the rendered CRDs
# with the ones from the config/crd/base directory. Any difference causes a non-zero return code.
.PHONY: crd-manifests-diff
crd-manifests-diff: crdgen
	@echo "## Checking if CRD manifests changed"
	$(eval PROTOBUF_MOD_PATH := $(shell go mod download --json github.com/gogo/protobuf | awk -F: '/"Dir"/ { print $$2 }' | tr -d ' ",'))
	$(eval CRD_OUT_PATH := $(shell mktemp -d))

	for proto in $(PROTOS); do \
		protoc \
			-I=../../api/proto/ \
			-I=$(PROTOBUF_MOD_PATH) \
			--plugin=./build/protoc-gen-crd \
			--crd_out=$(CRD_OUT_PATH) \
			"$${proto}"; \
	done

	diff config/crd/bases $(CRD_OUT_PATH);\
	EXIT_CODE=$$?;\
	rm -rf $(CRD_OUT_PATH) $(CUSTOM_IMPORTS_TMP_DIR);\
	exit $$EXIT_CODE

# This is a helper to dump a request sent to crdgen.
# See crdgen/DEBUG.md for instruction on how to use this target to replay crdgen with a debugger attached.
.PHONY: crdgen-debug-dump-request
debug-dump-request:
	$(eval PROTOBUF_MOD_PATH := $(shell go mod download --json github.com/gogo/protobuf | awk -F: '/"Dir"/ { print $$2 }' | tr -d ' ",'))
	for proto in $(PROTOS); do \
		protoc \
			-I=../../../api/proto/ \
			-I=$(PROTOBUF_MOD_PATH) \
			--plugin=./crdgen/hack/protoc-gen-dump \
			--dump_out="." \
			"$${proto}"; \
	done

.PHONY: test
test: generate $(ENVTEST) ## Run tests.
test: export KUBEBUILDER_ASSETS=$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)
test:
	go test ./... -coverprofile cover.out

.PHONY: echo-kubebuilder-assets
echo-kubebuilder-assets:
	@echo KUBEBUILDER_ASSETS=$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)


##@ Build

.PHONY: build
build: generate ## Build manager binary.
	go build -trimpath -tags "kustomize_disable_go_plugin_support" -o bin/manager main.go namespace.go config.go

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	docker buildx build --platform="$(OS)/$(ARCH)" \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		--build-arg PROTOC_VERSION=$(PROTOC_VERSION) \
		--build-arg BUILDARCH=$(ARCH) \
		--build-arg TARGETARCH=$(TARGETARCH) \
		--build-arg COMPILER_NAME=$(COMPILER) -t ${IMG} --load ../.. -f ./Dockerfile

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	docker push ${IMG}

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen

## Tool Versions
KUSTOMIZE_VERSION ?= v3.8.7
CONTROLLER_TOOLS_VERSION ?= v0.16.2

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary.
$(CONTROLLER_GEN): $(LOCALBIN)
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)
