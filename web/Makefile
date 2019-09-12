IMAGE_NAME := web-apps:1.0.0
REPO_VER_TAG := $(shell git describe --tags --abbrev=8)
CONTAINER_NAME := web-apps-container-$(REPO_VER_TAG)-$(shell bash -c 'echo $$RANDOM')
HOME_DIR = $(shell pwd)

.PHONY:gravity
gravity:
	$(MAKE) docker-build PACKAGE_PATH=packages/gravity NPM_CMD=build-gravity

.PHONY:teleport
teleport:
	$(MAKE) docker-build PACKAGE_PATH=packages/teleport NPM_CMD=build-teleport

.PHONY:docker-enter
docker-enter:
	docker run -ti --rm=true -t $(IMAGE_NAME) /bin/bash

.PHONY: docker-clean
docker-clean:
	docker rmi --force $(IMAGE_NAME)

docker-build:
	rm -rf $(HOME_DIR)/$(PACKAGE_PATH)/dist
	docker build --force-rm=true --build-arg NPM_SCRIPT=$(NPM_CMD) -t $(IMAGE_NAME) .
	docker create --name $(CONTAINER_NAME) -t $(IMAGE_NAME) && \
	docker cp $(CONTAINER_NAME):/web-apps/$(PACKAGE_PATH)/dist $(HOME_DIR)/$(PACKAGE_PATH)/
	docker rm -f $(CONTAINER_NAME)

.PHONY:clean
clean:
	find . -name "node_modules" -type d -prune -exec rm -rf '{}' +
	find . -name "dist" -type d -prune -exec rm -rf '{}' +

.PHONY:install
install:
	bash -c "./scripts/install.sh"

# Use git submodule status to figure out if submodule is not initialized
# (- at the beginning) or somehow modified/outdated (+ at the beginning).
# or the folder has been deleted ([null])
# .PHONY: ensure-submodules
# ensure-submodules:
# 	@if git submodule status | egrep -q '(^[-]|^[+])|[null]' ; then \
# 		git submodule update --init; \
# 	fi

.PHONY: init
init:
	git config core.hooksPath $(HOME_DIR)/scripts/githooks/oss

.PHONY: init-enterprise
init-enterprise:
	git config core.hooksPath $(HOME_DIR)/scripts/githooks/enterpise


#ensure-submodules
#$(shell test -d $(FMC_DRV) || git submodule update --init)