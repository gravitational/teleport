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
crd: crdgen crd-docs manifests

.PHONY: crdgen
crdgen: ## Generate CRDs
	make -C crdgen

PROTOS = \
	teleport/loginrule/v1/loginrule.proto \
	teleport/accesslist/v1/accesslist.proto \
	teleport/legacy/types/types.proto

.PHONY: manifests
manifests: crdgen controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
# Generate CRDs for apis/resources (custom generation through protoc plugin).

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

.PHONY: build-docs-plugin
build-docs-plugin: ## Build the protoc plugin for generating docs
	make -C crdgen build-docs

.PHONY: crd-docs
crd-docs: build-docs-plugin ## Generate CRD reference docs.

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

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet $(ENVTEST) crdgen-test ## Run tests.
test: export KUBEBUILDER_ASSETS=$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)
test:
	go test ./... -coverprofile cover.out

.PHONY: echo-kubebuilder-assets
echo-kubebuilder-assets:
	@echo KUBEBUILDER_ASSETS=$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) -p path)

.PHONY: crdgen-test
crdgen-test: ## Run crdgen tests.
	make -C crdgen test

##@ Build

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -trimpath -tags "kustomize_disable_go_plugin_support" -o bin/manager main.go namespace.go config.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./main.go ./namespace.go

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
