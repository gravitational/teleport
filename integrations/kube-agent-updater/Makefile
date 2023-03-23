# Image URL to use all building/pushing image targets
IMG ?= teleport-kube-agent-updater:latest

# include BUILDBOX_VERSION, BUILDBOX and BUILDBOX_variant variables
include ../../build.assets/images.mk

# Configure which compiler and buildbox to use
OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
ifeq ("$(OS)","linux")
ifeq ("$(ARCH)","amd64")
COMPILER ?= x86_64-linux-gnu-gcc
PLATFORM_BUILDBOX ?= $(BUILDBOX)
else ifeq ("$(ARCH)","386")
COMPILER ?= x86_64-linux-gnu-gcc
PLATFORM_BUILDBOX ?= $(BUILDBOX)
else ifeq ("$(ARCH)","arm")
COMPILER ?= arm-linux-gnueabihf-gcc
PLATFORM_BUILDBOX ?= $(BUILDBOX_ARM)
else ifeq ("$(ARCH)","arm64")
COMPILER ?= aarch64-linux-gnu-gcc
PLATFORM_BUILDBOX ?= $(BUILDBOX_ARM)
endif
endif

.PHONY: test
test: pkg/img/cosign_fixtures_test.go
	go test ./...

.PHONY: docker-build
docker-build: ## Build docker image
	docker buildx build --platform="$(OS)/$(ARCH)" --build-arg BUILDBOX=$(PLATFORM_BUILDBOX) \
		--build-arg COMPILER_NAME=$(COMPILER) -t ${IMG} --load ./ -f ./Dockerfile

.PHONY: docker-push
docker-push: ## Push docker image
	docker push ${IMG}

pkg/img/cosign_fixtures_test.go: hack/cosign-fixtures.go
	go run hack/cosign-fixtures.go | gofmt > pkg/img/cosign_fixtures_test.go
