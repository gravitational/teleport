# include BUILDBOX_VERSION, BUILDBOX and BUILDBOX_variant variables
include ../../../build.assets/images.mk

VERSION ?= $(shell go run ../../hack/get-version/get-version.go)
BUILDDIR ?= build
BINARY = $(BUILDDIR)/teleport-msteams
ADDFLAGS ?=
BUILDFLAGS ?= $(ADDFLAGS) -ldflags "-w -s"
CGOFLAG ?= CGO_ENABLED=0

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
RELEASE_NAME=teleport-access-msteams
RELEASE=$(RELEASE_NAME)-$(VERSION)-$(OS)-$(ARCH)-bin

RELEASE_MESSAGE = "Building with GOOS=$(OS) GOARCH=$(ARCH)."

DOCKER_VERSION = $(subst +,_,$(VERSION))
DOCKER_NAME = teleport-plugin-msteams
DOCKER_PRIVATE_REGISTRY = 146628656107.dkr.ecr.us-west-2.amazonaws.com
DOCKER_IMAGE = $(DOCKER_PRIVATE_REGISTRY)/gravitational/$(DOCKER_NAME):$(DOCKER_VERSION)
DOCKER_ECR_PUBLIC_REGISTRY = public.ecr.aws/gravitational
DOCKER_IMAGE_ECR_PUBLIC = $(DOCKER_ECR_PUBLIC_REGISTRY)/$(DOCKER_NAME):$(DOCKER_VERSION)
DOCKER_BUILD_ARGS = --load --platform="$(OS)/$(ARCH)" --build-arg ACCESS_PLUGIN=msteams --build-arg BUILDBOX=$(BUILDBOX)

.PHONY: $(BINARY)
$(BINARY):
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -o $(BINARY) $(BUILDFLAGS) github.com/gravitational/teleport/integrations/access/msteams/cmd/teleport-msteams

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
release: clean $(BINARY)
	@echo "---> $(RELEASE_MESSAGE)"
	mkdir build/$(RELEASE_NAME)
	cp -rf $(BINARY) \
		README.md \
		cmd/teleport-msteams/install \
		build/$(RELEASE_NAME)/
	echo $(VERSION) > build/$(RELEASE_NAME)/VERSION
	tar -czf build/$(RELEASE).tar.gz build/$(RELEASE_NAME)
	rm -rf build/$(RELEASE_NAME)/
	@echo "---> Created build/$(RELEASE).tar.gz."

.PHONY: docker-build
docker-build: ## Build docker image with the plugin.
	docker buildx build ${DOCKER_BUILD_ARGS} -t ${DOCKER_IMAGE} -f ../Dockerfile ../../..

.PHONY: docker-push
docker-push: ## Push docker image with the plugin.
	docker push ${DOCKER_IMAGE}
