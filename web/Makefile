# Some often referenced variables are declared below, to avoid repetition
IMAGE_NAME := web-apps
CONTAINER_NAME := web-apps-container-$(shell bash -c 'echo $$RANDOM')
ROOT = $(shell pwd)
BRANCH = $(shell git rev-parse --abbrev-ref HEAD)
COMMIT = $(shell git rev-parse --short HEAD)

# Below, we specify which files make up the source a particular library. We will later use those variables as
# dependencies for each target
COMMON_SRC = $(shell find packages -type f -a \( -path 'packages/build/*' -o -path 'packages/design/*' -o -path 'packages/shared/*' \))
COMMON_E_SRC = $(shell find packages/webapps.e/shared -type f 2> /dev/null)
FORCE_SRC = $(COMMON_SRC) $(shell find packages/force -type f -not -path  '*/dist/*')
GRAVITY_SRC = $(COMMON_SRC) $(shell find packages/gravity -type f -not -path  '*/dist/*')
GRAVITY_E_SRC = $(COMMON_E_SRC) $(GRAVITY_SRC) $(shell find packages/webapps.e/gravity -type f -not -path  '*/dist/*' 2> /dev/null)
TELEPORT_SRC = $(COMMON_SRC) $(shell find packages/teleport -type f -not -path  '*/dist/*')
TELEPORT_E_SRC = $(COMMON_E_SRC) $(TELEPORT_SRC) $(shell find packages/webapps.e/teleport -type f -not -path  '*/dist/*' 2> /dev/null)


# all is the default target which compiles all packages
.PHONY: all
all: packages/gravity/dist packages/teleport/dist packages/webapps.e/teleport/dist packages/webapps.e/gravity/dist
all: packages/force/dist


# The next few recipes are all instructions on how to build a specific target. As a reminder, Makefile recipes have
# the following syntax:
#
# path_to_build_target: build_dependencies
# 	instructions_to_build_said_target
#
# I.e. the following rule is for building the path `packages/force/dist`, lists all the files we listed earlier
# in `FORCE_SRC` as dependencies for building this path, and has an instruction on how to achieve that (using
# the `docker-build` command.
packages/force/dist: $(FORCE_SRC)
	$(MAKE) docker-build PACKAGE_PATH=packages/force NPM_CMD=build-force

packages/gravity/dist: $(GRAVITY_SRC)
	$(MAKE) docker-build PACKAGE_PATH=packages/gravity NPM_CMD=build-gravity

packages/teleport/dist: $(TELEPORT_SRC)
	$(MAKE) docker-build PACKAGE_PATH=packages/teleport NPM_CMD=build-teleport

# The enterprise files are only build if the submodule is available
packages/webapps.e/gravity/dist: $(GRAVITY_E_SRC)
	@if [ -d "packages/webapps.e/gravity" ]; then \
		$(MAKE) docker-build PACKAGE_PATH=packages/webapps.e/gravity NPM_CMD=build-gravity-e; \
	fi;

packages/webapps.e/teleport/dist: $(TELEPORT_E_SRC)
	@if [ -d "packages/webapps.e/teleport" ]; then \
		$(MAKE) docker-build PACKAGE_PATH=packages/webapps.e/teleport NPM_CMD=build-teleport-e; \
	fi;


# docker-build lists the common instructions on how to build one of our targets using docker. See the "real" targets,
# such as `packages/gravity/dist` on how to invoke this.
# Calling this target without providing the necessary variables (PACKAGE_PATH and NPM_CMD) will result in errors.
.PHONY: docker-build
docker-build:
	rm -rf $(ROOT)/$(PACKAGE_PATH)/dist
	docker build --force-rm=true --build-arg NPM_SCRIPT=$(NPM_CMD) -t $(IMAGE_NAME) .
	docker create --name $(CONTAINER_NAME) -t $(IMAGE_NAME) && \
	docker cp $(CONTAINER_NAME):/web-apps/$(PACKAGE_PATH)/dist $(ROOT)/$(PACKAGE_PATH)/
	docker rm -f $(CONTAINER_NAME)

# docker-enter is a shorthand for entering the build image, for example for debugging, or in case yarn cannot
# be used locally
.PHONY: docker-enter
docker-enter:
	docker run -ti --rm=true -t $(IMAGE_NAME) /bin/bash

# docker-clean removes the existing image
.PHONY: docker-clean
docker-clean:
	docker rmi --force $(IMAGE_NAME)


# deploy uploads the latest build artifacts for deployment. This rule is usually invoked by our CI servers,
# but may be used manually if your account has the right permissions.
# In essence, this target aggregates the various build artifacts that we compiled earlier & pushes them to a
# special place.
.PHONY: deploy
deploy: dist packages/webapps.e/dist
	@if [ "$(shell git --git-dir dist/.git rev-parse --abbrev-ref HEAD)" != "$(BRANCH)" ]; then \
		echo "Branch has changed since compilation, please run 'make clean' "; exit 2; \
	fi;
	@if [ "$(shell git --git-dir packages/webapps.e/dist/.git rev-parse --abbrev-ref HEAD)" != "$(BRANCH)" ]; then \
		echo "Branch has changed since compilation, please run 'make clean'"; exit 2; \
	fi;
	@if [ -d "packages/webapps.e/dist" ]; then \
		cd packages/webapps.e/dist; git add -A; git commit -am 'Update build artifacts from $(COMMIT)'; \
		git push; \
	fi;
	cd dist; git add -A; git commit -am 'Update build artifacts from $(COMMIT)'; git push

# Note this is not creating a tar file, thus deviating from the GNU coding standards for Makefiles
dist: packages/gravity/dist packages/teleport/dist packages/force/dist
	rm -rf dist
	git clone git@github.com:gravitational/webassets.git dist
	cd dist; git checkout $(BRANCH) || git checkout -b $(BRANCH)
	rm -rf dist/force dist/gravity dist/teleport
	mkdir -p dist/force && cp -r packages/force/dist/* dist/force
	mkdir -p dist/gravity && cp -r packages/gravity/dist/* dist/gravity
	mkdir -p dist/teleport && cp -r packages/teleport/dist/* dist/teleport

packages/webapps.e/dist: packages/webapps.e/teleport/dist packages/webapps.e/gravity/dist 
	rm -rf packages/webapps.e/dist
	git clone git@github.com:gravitational/webassets.e.git packages/webapps.e/dist
	cd packages/webapps.e/dist; git checkout $(BRANCH) || git checkout -b $(BRANCH)
	rm -rf packages/webapps.e/dist/gravity.e packages/webapps.e/dist/teleport.e
	mkdir -p packages/webapps.e/dist/gravity.e && cp -r packages/webapps.e/gravity/dist/* packages/webapps.e/dist/gravity.e
	mkdir -p packages/webapps.e/dist/teleport.e && cp -r packages/webapps.e/teleport/dist/* packages/webapps.e/dist/teleport.e


# check runs the test suite
.PHONY: check
check: all
	docker build --force-rm=true --build-arg NPM_SCRIPT=test -t $(IMAGE_NAME)-test .

# clean removes files that can be generated by this Makefile
.PHONY: clean
clean:
	rm -rf packages/gravity/dist packages/teleport/dist packages/force/dist
	rm -rf packages/webapps.e/gravity/dist packages/webapps.e/teleport/dist

# distcleans removes all files that are not part of the repository
.PHONY: distclean
distclean: clean
	find . -name "node_modules" -type d -prune -exec rm -rf '{}' +


# Some non-standard targets:

# Some of our other projects use `make test` to launch the test suite
.PHONY: test
test: check

# install installs npm / yarn dependencies
# Note this is not a typical `install` targets, as with most other Makefiles
.PHONY:install
install:
	bash -c "./scripts/install.sh"

# init-submodules initializes / updates the submodules in this repo
.PHONY: init-submodules
init-submodules:
	git submodule update --init --recursive

