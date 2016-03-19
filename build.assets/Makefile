#
# This Makefile is used for producing official Teleport releases
#
BBOX=teleport-buildbox:latest
SRCDIR=/gopath/src/github.com/gravitational/teleport
DOCKERFLAGS=-i --rm=true -v "$$(pwd)/../":$(SRCDIR) -w $(SRCDIR)
BUILDFLAGS=-ldflags -w
NOROOT=-u $$(id -u):$$(id -g)

#
# Build 'teleport' binaries
#
.PHONY:build
build: bbox
	docker run $(DOCKERFLAGS) $(NOROOT) $(BBOX) \
		/bin/bash -c "$(MAKE) -C $(SRCDIR) BUILDFLAGS='$(BUILDFLAGS)' all"

#
# Builds a Docker container which is used for building official Teleport 
# binaries and docs
#
.PHONY:bbox
bbox:
	docker build --build-arg UID=$$(id -u) --build-arg GID=$$(id -g) --tag $(BBOX) .


#
# Runs tests inside a build container 
#
.PHONY:test
test:
	docker run $(DOCKERFLAGS) $(NOROOT) -t $(BBOX) \
		/bin/bash -c "$(MAKE) -C $(SRCDIR) TELEPORT_DEBUG_TESTS=true FLAGS='-cover -race' clean test"


#
# Builds docs
#
.PHONY:docs
docs: bbox
	docker run $(DOCKERFLAGS) -ti $(NOROOT) -e HOME=$(SRCDIR)/build.assets $(BBOX) mkdocs build

#
# Starts shell inside the build container
#
.PHONY:enter
enter: bbox
	docker run $(DOCKERFLAGS) -ti $(NOROOT) $(BBOX) /bin/bash
