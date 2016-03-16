#
# This Makefile is used for CI/CD builds. Please be careful!
#
BBOX=teleport-buildbox:latest
SRCDIR=/gopath/src/github.com/gravitational/teleport
DOCKERFLAGS=-i --rm=true \
	-v "$$(pwd)/../":$(SRCDIR) \
	-u $$(id -u):$$(id -g)
BUILDFLAGS=-ldflags -w

#
# builds 'teleport' binary
#
.PHONY:build
build: bbox
	docker run $(DOCKERFLAGS) $(BBOX) \
		/bin/bash -c "$(MAKE) -C $(SRCDIR) BUILDFLAGS='$(BUILDFLAGS)' all"

#
# builds buildbox:1.0.0 Docker container which is Debian8 with Golang 1.4.2
#
.PHONY:bbox
bbox:
	docker build --tag $(BBOX) .


#
# runs tests inside a build container 
#
.PHONY:test
test:
	docker run $(DOCKERFLAGS) $(BBOX) \
		/bin/bash -c "$(MAKE) -C $(SRCDIR) TELEPORT_DEBUG_TESTS=true FLAGS='-cover -race' clean test"

#
# starts teleport inside a build container
#
.PHONY:run
run: build
	docker run $(DOCKERFLAGS) $(BBOX) $(SRCDIR)/out/teleport start -d
