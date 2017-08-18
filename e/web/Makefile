TAG=1.0.0
HOST_DIR="$$(pwd)"
DIST_DIR=$(HOST_DIR)/dist
TELEBASE:=telebase
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
	docker run -v $(TELEBASE_SRCDIR):/teleport/web -v $(DIST_DIR):/teleport/e/web/dist $(NOROOT) -t $(IMAGE_NAME) make local
		
#
# 'make docker-enter' builds a Docker container with a website
#
.PHONY:docker-enter
docker-enter:
	docker run -ti --rm=true -t $(IMAGE_NAME) /bin/bash


.PHONY:ensure-deps
ensure-deps:		
	rm -rf $(TELEBASE)
	mkdir -p $(TELEBASE)/
	cp -R $(TELEBASE_SRCDIR)/src/. $(TELEBASE)/

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