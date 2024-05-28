# Image URL to use all building/pushing image targets
IMG ?= teleport-kube-agent-updater:latest

# include BUILDBOX_VERSION, BUILDBOX and BUILDBOX_variant variables
include ../../build.assets/images.mk

# Configure which compiler and buildbox to use
OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

.PHONY: test
test: pkg/img/cosign_fixtures_test.go
	go test ./...

.PHONY: docker-build
docker-build: ## Build docker image
	docker buildx build --platform="$(OS)/$(ARCH)" --build-arg BUILDBOX=$(BUILDBOX) \
		-t ${IMG} --load ../.. -f ./Dockerfile

.PHONY: docker-push
docker-push: ## Push docker image
	docker push ${IMG}

pkg/img/cosign_fixtures_test.go: hack/cosign-fixtures.go
	go run hack/cosign-fixtures.go | gofmt > pkg/img/cosign_fixtures_test.go
