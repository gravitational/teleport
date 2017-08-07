TAG=1.0.0
HOST_DIR="$$(pwd)"
DIST_DIR=$(HOST_DIR)/dist
TELEBASE_SRCDIR=$(HOST_DIR)/../../web
NOROOT=-u $$(id -u):$$(id -g)
IMAGE_NAME:=e-teleport-web:$(TAG)

.PHONY:all
all: docker-build

#
# Builds dist files
#
.PHONY:docker-build
docker-build:	
	rm -rf $(DIST_DIR)
	mkdir -p $(DIST_DIR) 	
	docker build --build-arg UID=$$(id -u) --build-arg GID=$$(id -g) --force-rm=true -t $(IMAGE_NAME) -f Dockerfile . 		
	docker run -v $(TELEBASE_SRCDIR):/teleport/web -v $(DIST_DIR):/teleport/e/web/dist $(NOROOT) -t $(IMAGE_NAME) npm run build 
		
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
local: 
	npm install
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