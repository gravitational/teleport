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
		$(MAKE) docker-build NPM_CMD=build FROM=dist/ TO=dist/; \
	else \
		$(MAKE) docker-build NPM_CMD=build-oss FROM=dist/ TO=dist/; \
	fi;

.PHONY: test
test:
	$(MAKE) docker-build NPM_CMD=test

.PHONY: build-force
build-force:
	$(MAKE) docker-build NPM_CMD=build-force FROM=dist/force/ TO=dist/force

.PHONY: build-gravity-oss
build-gravity-oss:
	$(MAKE) docker-build NPM_CMD=build-gravity-oss FROM=dist/gravity/ TO=dist/gravity

.PHONY: build-teleport-oss
build-teleport-oss:
	$(MAKE) docker-build NPM_CMD=build-teleport-oss FROM=dist/teleport/ TO=dist/teleport

.PHONY: build-gravity-e
build-gravity-e:
	$(MAKE) docker-build NPM_CMD=build-gravity-e FROM=dist/e/gravity/ TO=dist/e/gravity;

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

# update teleport repository with build assets
.PHONY: update-teleport-repo
update-teleport-repo:
	@if [ -z "$(TELEPORT_TARGET)" ]; then \
		echo "TELEPORT_TARGET is not set"; exit 1; \
	fi
	# prepare webassets repo
	rm -rf dist && git clone git@github.com:gravitational/webassets.git dist
	cd dist; git checkout $(BRANCH) || git checkout -b $(BRANCH)
	cd dist; rm -fr */
	# prepare webassets.e repo
	cd dist; git submodule update --init --recursive
	cd dist/e; git checkout $(BRANCH) || git checkout -b $(BRANCH)
	cd dist/e; rm -fr */
	# build the dist files
	$(MAKE) build-teleport
	# push dist files to webasset/e repositories
	cd dist/e; git add -A .; git commit -am '$(COMMIT_DESC)' -m '$(COMMIT_URL)' --allow-empty; git push origin $(BRANCH)
	cd dist; git add -A .; git commit -am '$(COMMIT_DESC)' -m '$(COMMIT_URL)' --allow-empty; git push origin $(BRANCH)
	# use temporary file to store new webassets commit sha
	cd dist; git rev-parse HEAD >> commit_sha;
	# update teleport
	echo teleport >> .gitignore
	rm -rf teleport && git clone git@github.com:gravitational/teleport.git
	cd teleport; git checkout $(TELEPORT_TARGET) || git checkout -b $(TELEPORT_TARGET)
	cd teleport; git fetch --recurse-submodules && git submodule update --init webassets
	cd teleport/webassets; git checkout $$(cat -v ../../dist/commit_sha)
	cd teleport; git add -A .; git commit -am 'Update webassets' -m '$(COMMIT_DESC) $(COMMIT_URL)' --allow-empty
	cd teleport; git push origin $(TELEPORT_TARGET)

# clean removes this repo generated artifacts
.PHONY: clean
clean:
	rm -rf dist teleport
	find . -name "node_modules" -type d -prune -exec rm -rf '{}' +

# init-submodules initializes / updates the submodules in this repo
.PHONY: init-submodules
init-submodules:
	git submodule update --init --recursive

