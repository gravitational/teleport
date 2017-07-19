TAG=1.0.0
NOROOT=-u $$(id -u):$$(id -g)
LOCALDIR := $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST)))

CONTAINER_DIST_DIR=/app/dist
HOST_DIR="$$(pwd)"
HOST_TMP_VOL:= $(shell mktemp -d "/tmp/teleport-build-dist.XXXXXXXXX")

GRV_TELEBASE := telebase
GRV_TELEBASE_LOC := $(LOCALDIR)/$(GRV_TELEBASE)
GRV_TELEBASE_REPO := git@github.com:gravitational/teleport.git
GRV_TELEBASE_TAG ?= master

.PHONY:all
all: docker-build

.PHONY:revendor
revendor:		
	rm -rf $(GRV_TELEBASE)
	mkdir -p $(GRV_TELEBASE_LOC)/web
	cp -R $(GOPATH)/src/github.com/gravitational/teleport/web/src $(GRV_TELEBASE_LOC)/web

.PHONY:git-revendor
git-revendor:		
	rm -rf $(GRV_TELEBASE)
	if [ ! -d "$(GRV_TELEBASE_LOC)" ]; then \
		mkdir -p $(GRV_TELEBASE_LOC);	\
		cd $(GRV_TELEBASE_LOC) && \
		git init && \
		git remote add origin $(GRV_TELEBASE_REPO) && \
		git config core.sparsecheckout true && \
		echo "web/src/*" >> .git/info/sparse-checkout; \
	fi;
	cd $(GRV_TELEBASE_LOC) &&	git pull --depth=1 origin $(GRV_TELEBASE_TAG)
	rm -rf $(GRV_TELEBASE_LOC)/.git

#
# Builds dist files
#
.PHONY:docker-build
docker-build:
	rm -rf $(HOST_DIR)/dist
	docker build --force-rm=true -t e-teleport-web:$(TAG) -f Dockerfile . 
	
#
# 'make docker-enter' builds a Docker container with a website
#
.PHONY:docker-enter
docker-enter:
	docker run -ti --rm=true $(NOROOT) -t e-teleport-web:$(TAG) /bin/bash

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
run:
	npm run start

.PHONY:clean
clean:	
	rm -rf dist
	rm -rf node_modules	