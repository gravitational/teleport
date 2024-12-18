# include BUILDBOX_VERSION, BUILDBOX and BUILDBOX_variant variables
# the include path is relative to the including Makefile (nested one directory deeper) not to this one.
include ../../../build.assets/images.mk

include ../../../build.assets/relcli.mk

VERSION ?= $(shell go run ../../hack/get-version/get-version.go)
BUILDDIR ?= build
RELEASE_DIR ?= build
BINARY = $(BUILDDIR)/teleport-$(ACCESS_PLUGIN)
ADDFLAGS ?=
BUILDFLAGS ?= $(ADDFLAGS) -trimpath -ldflags "-w -s"
CGOFLAG ?= CGO_ENABLED=0

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
RELEASE_NAME=teleport-access-$(ACCESS_PLUGIN)
RELEASE=$(RELEASE_NAME)-v$(VERSION)-$(OS)-$(ARCH)-bin

RELEASE_MESSAGE = "Building with GOOS=$(OS) GOARCH=$(ARCH)."

PLUGIN_NAME ?= $(ACCESS_PLUGIN)

DOCKER_VERSION = $(subst +,_,$(VERSION))
DOCKER_NAME = teleport-plugin-$(ACCESS_PLUGIN)
DOCKER_PRIVATE_REGISTRY = 146628656107.dkr.ecr.us-west-2.amazonaws.com
DOCKER_IMAGE_BASE = $(DOCKER_PRIVATE_REGISTRY)/gravitational
DOCKER_IMAGE = $(DOCKER_IMAGE_BASE)/$(DOCKER_NAME):$(DOCKER_VERSION)
DOCKER_ECR_PUBLIC_REGISTRY = public.ecr.aws/gravitational
DOCKER_IMAGE_ECR_PUBLIC = $(DOCKER_ECR_PUBLIC_REGISTRY)/$(DOCKER_NAME):$(DOCKER_VERSION)
DOCKER_BUILD_ARGS = --load --platform="$(OS)/$(ARCH)"
# In staging
# DOCKER_PRIVATE_REGISTRY = 603330915152.dkr.ecr.us-west-2.amazonaws.com
# DOCKER_ECR_PUBLIC_REGISTRY = public.ecr.aws/gravitational-staging

.PHONY: $(BINARY)
$(BINARY):
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -o $(BINARY) $(BUILDFLAGS) github.com/gravitational/teleport/integrations/access/$(ACCESS_PLUGIN)/cmd/teleport-$(ACCESS_PLUGIN)

.PHONY: test
test: FLAGS ?= '-race'
test:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go test $(FLAGS) $(ADDFLAGS)

clean:
	@echo "---> Cleaning up build artifacts."
	rm -rf $(BUILDDIR)
	rm -rf $(RELEASE_NAME)
	rm -rf *.gz

.PHONY: release
release: $(BINARY) $(RELCLI)
	@echo "---> $(RELEASE_MESSAGE)"
	mkdir build/$(RELEASE_NAME)
	cp -rf $(BINARY) \
		README.md \
		cmd/teleport-$(ACCESS_PLUGIN)/install \
		build/$(RELEASE_NAME)/
	echo $(VERSION) > build/$(RELEASE_NAME)/VERSION
	tar -C build/ -czf $(RELEASE_DIR)/$(RELEASE).tar.gz $(RELEASE_NAME)
	rm -rf build/$(RELEASE_NAME)/
	@echo "---> Created $(RELEASE_DIR)/$(RELEASE).tar.gz."
	$(RELCLI) generate-manifest \
        --path "$(RELEASE_DIR)/$(RELEASE).tar.gz" \
		--os "$(OS)" \
		--architecture "$(ARCH)" \
        --description "$(PLUGIN_NAME) plugin for Teleport"

.PHONY: docker-build
docker-build: OS = linux
docker-build: release ## Build docker image with the plugin.
	docker buildx build ${DOCKER_BUILD_ARGS} -t ${DOCKER_IMAGE} -f ../Dockerfile ./build

.PHONY: docker-push
docker-push: ## Push docker image with the plugin.
	docker push ${DOCKER_IMAGE}

.PHONY: docker-publish
docker-publish: ## Publishes a docker image from the private ECR registry to the public one.
	docker pull ${DOCKER_IMAGE}
	docker tag ${DOCKER_IMAGE} ${DOCKER_IMAGE_ECR_PUBLIC}
	docker push ${DOCKER_IMAGE_ECR_PUBLIC}
