#
# This Makefile is used for producing official Teleport releases
#
BBOX=teleport-buildbox:latest
HOSTNAME=buildbox
SRCDIR=/gopath/src/github.com/gravitational/teleport
DOCKERFLAGS=--rm=true -v "$$(pwd)/../":$(SRCDIR) -w $(SRCDIR) -h $(HOSTNAME)
ADDFLAGS=-ldflags -w
NOROOT=-u $$(id -u):$$(id -g)

#
# Build 'teleport' binaries
#
.PHONY:build
build: bbox
	docker run $(DOCKERFLAGS) $(NOROOT) $(BBOX) \
		make -C $(SRCDIR) ADDFLAGS='$(ADDFLAGS)' all

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
		/bin/bash -c "$(MAKE) -C $(SRCDIR) TELEPORT_DEBUG=1 FLAGS='-cover -race' clean test"


.PHONY:integration
integration: bbox
	docker run $(DOCKERFLAGS) $(NOROOT) -t $(BBOX) \
 		/bin/bash -c "$(MAKE) -C $(SRCDIR) FLAGS='-cover' integration"

#
# Builds docs
#
.PHONY:docs
docs: bbox
	docker run $(DOCKERFLAGS) -ti $(NOROOT) -e HOME=$(SRCDIR)/build.assets $(BBOX) mkdocs build
	@echo "\nSUCCESS: Teleport docs ----> build/docs\n"

#
# Runs docs website on localhost
#
.PHONY:run-docs
run-docs: bbox
	@echo -e "\n\n----> LIVE EDIT HERE: http://localhost:6600/admin-guide/\n"
	docker run $(DOCKERFLAGS) -ti $(NOROOT) -e HOME=$(SRCDIR)/build.assets -p 6600:6600 -w $(SRCDIR) $(BBOX) mkdocs serve -a 0.0.0.0:6600

#
# Starts shell inside the build container
#
.PHONY:enter
enter: bbox
	docker run $(DOCKERFLAGS) -ti $(NOROOT) $(BBOX) /bin/bash

#
# Create a teleport package using the build container
#
.PHONY:release
release: 
	docker run $(DOCKERFLAGS) -i $(NOROOT) $(BBOX) /usr/bin/make release -e VERSION="$(VERSION)" -e ADDFLAGS="$(ADDFLAGS)"
