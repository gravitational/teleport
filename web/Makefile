LOCALDIR := $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST)))
REPO_VER_TAG := $(shell git describe --tags --abbrev=8)
CONTAINER_DIST_DIR=/app/dist
# Need unique names to allow parallel jenkins builds
CONTAINER_NAME := teleport-web-container-$(REPO_VER_TAG)-$(shell bash -c 'echo $$RANDOM')
IMAGE_NAME := teleport-web:1.0.0

HOST_DIR="$$(pwd)"
#HOST_TMP_VOL:= $(shell mktemp -d)

.PHONY:all
all: docker-build

##############################
#   Docker Targets
##############################

#
# Builds dist files
#
.PHONY:docker-build
docker-build: 
	rm -rf $(HOST_DIR)/dist	
	docker build --force-rm=true -t $(IMAGE_NAME) . 
	docker create --name $(CONTAINER_NAME) -t $(IMAGE_NAME) && \
	docker cp $(CONTAINER_NAME):$(CONTAINER_DIST_DIR) $(HOST_DIR)
	docker rm $(CONTAINER_NAME) | true

#
# Removes the local Docker container
#
.PHONY: docker-clean
docker-clean:
	docker rmi --force $(IMAGE_NAME)

#
# 'make docker-enter' builds a Docker container with a website
#
.PHONY:docker-enter
docker-enter:
	docker run -ti --rm=true -t $(IMAGE_NAME) /bin/bash

##############################
#   Native Localhost
##############################

#
# locally installs npm packages and builds
#
.PHONY:local
local: npm install
	npm run build

#
# 'make run' starts a local debugging environment
#
.PHONY:run
run:
	npm run start

.PHONY:clean
clean:	
	rm -rf dist
	rm -rf node_modules	
