# Some often referenced variables are declared below, to avoid repetition
IMAGE_NAME := web-apps
CONTAINER_NAME := web-apps-container-$(shell bash -c 'echo $$RANDOM')
ROOT = $(shell pwd)
BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
COMMIT = $(shell git rev-parse --short HEAD)
COMMIT_DESC = $(shell git log --decorate=off --oneline -1)
COMMIT_URL = https://github.com/gravitational/webapps/commit/$(COMMIT)

.PHONY: build
build:
	@if [ -d "packages/webapps.e/" ]; then \
		$(MAKE) docker-build NPM_CMD=build-teleport FROM=dist/ TO=dist/; \
	else \
		$(MAKE) docker-build NPM_CMD=build-oss FROM=dist/ TO=dist/; \
	fi;

.PHONY: test
test:
	$(MAKE) docker-build NPM_CMD=test

.PHONY: build-teleport-oss
build-teleport-oss:
	$(MAKE) docker-build NPM_CMD=build-teleport-oss FROM=dist/teleport/ TO=dist/teleport

.PHONY: build-teleport-e
build-teleport-e:
	$(MAKE) docker-build NPM_CMD=build-teleport-e FROM=dist/e/teleport/ TO=dist/e/teleport;

.PHONY: build-teleport
build-teleport:
	$(MAKE) docker-build NPM_CMD=build-teleport FROM=dist/ TO=dist/;

# builds package dists files in docker (TODO: move it to scripts/docker-build.sh)
.PHONY: docker-build
docker-build:
	docker build --force-rm=true --build-arg NPM_SCRIPT=$(NPM_CMD) -t $(IMAGE_NAME) .
	@if [ "$(TO)" != "" ] || [ "$(FROM)" != "" ]; then \
		mkdir -p $(ROOT)/$(TO); \
		docker create --name $(CONTAINER_NAME) -t $(IMAGE_NAME) && \
		docker cp $(CONTAINER_NAME):/web-apps/$(FROM)/. $(ROOT)/$(TO); \
		docker rm -f $(CONTAINER_NAME); \
	fi;

# docker-enter is a shorthand for entering the image
.PHONY: docker-enter
docker-enter:
	docker run -ti --rm=true -t $(IMAGE_NAME) /bin/bash

# docker-clean removes the existing image
.PHONY: docker-clean
docker-clean:
	docker rmi --force $(IMAGE_NAME)

# update-teleport-repo has moved
# it now lives here: https://github.com/gravitational/ops/tree/master/webapps/update-teleport-webassets.sh

# clean removes this repo generated artifacts
.PHONY: clean
clean:
	rm -rf dist teleport
	find . -name "node_modules" -type d -prune -exec rm -rf '{}' +

# init-submodules initializes / updates the submodules in this repo
.PHONY: init-submodules
init-submodules:
	git submodule update --init --recursive
