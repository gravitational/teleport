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

.PHONY:teleport-e
teleport-e:
	$(MAKE) docker-build PACKAGE_PATH=packages/e/teleport NPM_CMD=build-teleport-e

.PHONY:gravity-e
gravity-e:
	$(MAKE) docker-build PACKAGE_PATH=packages/e/gravity NPM_CMD=build-gravity-e

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

.PHONY: init
init:
	git config core.hooksPath $(HOME_DIR)/scripts/githooks/oss

.PHONY: init-enterprise
init-enterprise:
	git config core.hooksPath $(HOME_DIR)/scripts/githooks/enterpise
