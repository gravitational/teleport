#
# This Makefile is used for producing official Teleport releases
#
BBOX=teleport-buildbox:latest
HOSTNAME=buildbox
SRCDIR=/gopath/src/github.com/gravitational/teleport
DOCKERFLAGS=--rm=true -v "$$(pwd)/../":$(SRCDIR) -v /tmp:/tmp -w $(SRCDIR) -h $(HOSTNAME)
ADDFLAGS=-ldflags -w
NOROOT=-u $$(id -u):$$(id -g)

#
# Build 'teleport' release inside a docker container
#
.PHONY:build
build: bbox
	docker run $(DOCKERFLAGS) $(NOROOT) $(BBOX) \
		make -C $(SRCDIR) ADDFLAGS='$(ADDFLAGS)' release

#
# Build 'teleport' release inside a docker container
#
.PHONY:build-binaries
build-binaries: bbox
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
test: bbox
	docker run $(DOCKERFLAGS) $(NOROOT) -t $(BBOX) \
		/bin/bash -c \
		"examples/etcd/start-etcd.sh & sleep 1; \
		ssh-agent > external.agent.tmp && source external.agent.tmp; \
		cd $(SRCDIR) && make TELEPORT_DEBUG=0 FLAGS='-cover -race' clean test"

.PHONY:integration
integration: bbox
	docker run $(DOCKERFLAGS) $(NOROOT) -t $(BBOX) \
 		/bin/bash -c "make -C $(SRCDIR) FLAGS='-cover' integration"

#
# Builds docs
#
.PHONY:docs
docs: bbox
	docker run $(DOCKERFLAGS) -ti $(NOROOT) \
		-e HOME=$(SRCDIR)/build.assets $(BBOX) \
		./docs/build.sh
	@echo "\nSUCCESS: Teleport docs ----> build/docs\n"

#
# Runs docs website on localhost
#
.PHONY:run-docs
run-docs: bbox
	@echo -e "\n\n----> LIVE EDIT HERE: http://localhost:6600/admin-guide/\n"
	docker run $(DOCKERFLAGS) -ti $(NOROOT) \
		-e HOME=$(SRCDIR)/build.assets \
		-p 6600:6600 \
		-w $(SRCDIR) \
		$(BBOX) docs/run.sh

#
# Starts shell inside the build container
#
.PHONY:enter
enter: bbox
	docker run $(DOCKERFLAGS) -ti $(NOROOT) \
		-e HOME=$(SRCDIR)/build.assets -w $(SRCDIR) $(BBOX) /bin/bash

#
# Create a teleport package using the build container
#
.PHONY:release
release:
	docker run $(DOCKERFLAGS) -i $(NOROOT) $(BBOX) \
		/usr/bin/make release -e ADDFLAGS="$(ADDFLAGS)"

