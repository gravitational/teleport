# Image URL to use all building/pushing image targets
IMG ?= teleport-spacelift-runner:latest
BASE_IMAGE ?= public.ecr.aws/spacelift/runner-terraform:v1.4.2_20231120

# Configure which compiler and buildbox to use
OS ?= $(shell go env GOOS)
# ARCH is the architecture of the current machine. For cross-compilation, use
# TARGETARCH instead.
ARCH ?= $(shell go env GOARCH)

# include BUILDBOX_VERSION, BUILDBOX and BUILDBOX_variant variables
include ../../build.assets/images.mk

.PHONY: docker-build
docker-build:
	docker buildx build --platform="$(OS)/$(ARCH)" \
		--build-arg BUILDBOX=$(BUILDBOX_CENTOS7_NOARCH) \
		--build-arg BASE_IMAGE=$(BASE_IMAGE) \
		--load \
		-f ./Dockerfile \
		-t $(IMG) \
		 ../..

.PHONY: docker-push
docker-push:
	docker push $(IMG)

.PHONY:show-base-image
show-base-image:
	@echo "$(BASE_IMAGE)"
