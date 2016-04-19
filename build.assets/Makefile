#
# This Makefile is used for producing official Teleport releases
#
BBOX=teleport-buildbox:latest
SRCDIR=/gopath/src/github.com/gravitational/teleport
DOCKERFLAGS=--rm=true -v "$$(pwd)/../":$(SRCDIR) -w $(SRCDIR)
ADDFLAGS=-ldflags -w
NOROOT=-u $$(id -u):$$(id -g)

#
# Build 'teleport' binaries
#
.PHONY:build
build: bbox
	docker run $(DOCKERFLAGS) $(NOROOT) $(BBOX) \
		/bin/bash -c "$(MAKE) -C $(SRCDIR) ADDFLAGS='$(ADDFLAGS)' all"

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
test: integration
	docker run $(DOCKERFLAGS) $(NOROOT) -t $(BBOX) \
		/bin/bash -c "$(MAKE) -C $(SRCDIR) TELEPORT_DEBUG_TESTS=1 FLAGS='-cover -race' clean test"


.PHONY:integration
integration: bbox
	docker run $(DOCKERFLAGS) $(NOROOT) -i $(BBOX) \
		/bin/bash -c "$(MAKE) -C $(SRCDIR) FLAGS='-cover' integration"

#
# Builds docs
#
.PHONY:docs
docs: bbox
	docker run $(DOCKERFLAGS) -ti $(NOROOT) -e HOME=$(SRCDIR)/build.assets $(BBOX) mkdocs build
	@echo "\nSUCCESS: Teleport docs ----> build/docs\n"

#
# Starts shell inside the build container
#
.PHONY:enter
enter: bbox
	docker run $(DOCKERFLAGS) -ti $(NOROOT) $(BBOX) /bin/bash
