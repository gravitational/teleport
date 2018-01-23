TAG=1.0.0
HOST_DIR="$$(pwd)"
DIST_DIR=$(HOST_DIR)/dist
TELEBASE:=telebase
TELEBASE_EXAMPLES:=telebase-examples
TELEBASE_SRCDIR=$(HOST_DIR)/../../web
TELEBASE_EXAMPLES_SRCDIR=$(HOST_DIR)/../../examples

REPO_VER_TAG := $(shell git describe --tags --abbrev=8)
CONTAINER_DIST_DIR=/teleport/e/web/dist
# Need unique names to allow parallel jenkins builds
CONTAINER_NAME := teleport-e-web-container-$(REPO_VER_TAG)-$(shell bash -c 'echo $$RANDOM')
IMAGE_NAME := teleport-e-web:1.0.0

.PHONY:all
all: docker-build

#
# Builds dist files
#
.PHONY:docker-build
docker-build: ensure-deps
	rm -rf $(DIST_DIR)
	mkdir -p $(DIST_DIR) 		
	docker build --force-rm=true -t $(IMAGE_NAME) . 
	docker create --name $(CONTAINER_NAME) -t $(IMAGE_NAME) && 	docker cp $(CONTAINER_NAME):$(CONTAINER_DIST_DIR) $(HOST_DIR)
	docker rm $(CONTAINER_NAME) | true
			
#
# 'make docker-enter' builds a Docker container with a website
#
.PHONY:docker-enter
docker-enter:
	docker run -ti --rm=true -t $(IMAGE_NAME) /bin/bash

.PHONY:ensure-deps
ensure-deps:		
	rm -rf $(TELEBASE)
	rm -rf $(TELEBASE_EXAMPLES)
	mkdir -p $(TELEBASE)/
	mkdir -p $(TELEBASE_EXAMPLES)/
	cp -R $(TELEBASE_SRCDIR)/src/. $(TELEBASE)/
	cp -R $(TELEBASE_EXAMPLES_SRCDIR)/. $(TELEBASE_EXAMPLES)/		

##############################
#   Native Localhost
##############################


#
# locally installs npm packages and builds
#
.PHONY:local
local: ensure-deps
	npm install
	npm run build

#
# 'make run' starts a local debugging environment
#
.PHONY:run
run: ensure-deps
	npm run start

.PHONY:clean
clean:	
	rm -rf dist
	rm -rf node_modules	
	rm -rf telebase	
	rm -rf telebase-examples