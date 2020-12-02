#
# This Makefile is used for producing official Teleport releases
#
DOCSBOX=teleport-docsbox:latest
DOCSHOST=teleport-docs
DOCSDIR=/teleport

HOSTNAME=buildbox
SRCDIR=/go/src/github.com/gravitational/teleport
DOCKERFLAGS := --rm=true -v "$$(pwd)/../":$(SRCDIR) -v /tmp:/tmp -w $(SRCDIR) -h $(HOSTNAME)
BCCFLAGS := -v "$$(pwd)/bcc:/usr/include/bcc"
ADDFLAGS=-ldflags -w
NOROOT=-u $$(id -u):$$(id -g)
KUBECONFIG ?=
TEST_KUBE ?=

OS ?= linux
ARCH ?= amd64
RUNTIME ?= go1.15.5

UID ?= $$(id -u)
GID ?= $$(id -g)

BUILDBOX=quay.io/gravitational/teleport-buildbox:$(RUNTIME)
BUILDBOXFIPS=quay.io/gravitational/teleport-buildbox-fips:$(RUNTIME)
BUILDBOXCENTOS6=quay.io/gravitational/teleport-buildbox-centos6:$(RUNTIME)
BUILDBOXCENTOS6FIPS=quay.io/gravitational/teleport-buildbox-centos6-fips:$(RUNTIME)

ifneq ("$(KUBECONFIG)","")
DOCKERFLAGS := $(DOCKERFLAGS) -v $(KUBECONFIG):/mnt/kube/config -e KUBECONFIG=/mnt/kube/config -e TEST_KUBE=$(TEST_KUBE)
endif

# conditionally force the use of UID/GID 1000:1000 if we're running in Drone
ifeq ("$(DRONE)","true")
UID := 1000
GID := 1000
NOROOT := -u 1000:1000
# pass external gocache path through to docker containers
DOCKERFLAGS := $(DOCKERFLAGS) -v $(GOCACHE):/go/cache -e GOCACHE=/go/cache
endif
export


#
# Build 'teleport' release inside a docker container
#
.PHONY:build
build: buildbox
	docker run $(DOCKERFLAGS) $(BCCFLAGS) $(NOROOT) $(BUILDBOX) \
		make -C $(SRCDIR) ADDFLAGS='$(ADDFLAGS)' release

#
# Build 'teleport' release inside a docker container
#
.PHONY:build-binaries
build-binaries: buildbox
	docker run $(DOCKERFLAGS) $(BCCFLAGS) $(NOROOT) $(BUILDBOX) \
		make -C $(SRCDIR) ADDFLAGS='$(ADDFLAGS)' full

#
# Build 'teleport' Enterprise release inside a docker container
#
.PHONY:build-enterprise-binaries
build-enterprise-binaries: buildbox
	docker run $(DOCKERFLAGS) $(BCCFLAGS) $(NOROOT) $(BUILDBOX) \
		make -C $(SRCDIR)/e ADDFLAGS='$(ADDFLAGS)' VERSION=$(VERSION) GITTAG=v$(VERSION) full

#
# Build 'teleport' FIPS release inside a docker container
# This builds Enterprise binaries only.
#
.PHONY:build-binaries-fips
build-binaries-fips: buildbox-fips
	docker run $(DOCKERFLAGS) $(BCCFLAGS) $(NOROOT) $(BUILDBOXFIPS) \
		make -C $(SRCDIR)/e ADDFLAGS='$(ADDFLAGS)' VERSION=$(VERSION) GITTAG=v$(VERSION) FIPS=yes full

#
# Builds a Docker container which is used for building official Teleport
# binaries and docs
#
.PHONY:buildbox
buildbox:
	docker build \
		--build-arg UID=$(UID) \
		--build-arg GID=$(GID) \
		--build-arg RUNTIME=$(RUNTIME) \
		--cache-from $(BUILDBOX) \
		--tag $(BUILDBOX) .

#
# Builds a Docker buildbox for FIPS
#
.PHONY:buildbox-fips
buildbox-fips:
	docker build \
		--build-arg UID=$(UID) \
		--build-arg GID=$(GID) \
		--build-arg RUNTIME=$(RUNTIME) \
		--cache-from $(BUILDBOXFIPS) \
		--tag $(BUILDBOXFIPS) -f Dockerfile-fips .

#
# Builds a Docker container for CentOS 6 builds
#
.PHONY:buildbox-centos6
buildbox-centos6:
	docker build \
		--build-arg UID=$(UID) \
		--build-arg GID=$(GID) \
		--build-arg RUNTIME=$(RUNTIME) \
		--cache-from $(BUILDBOXCENTOS6) \
		--tag $(BUILDBOXCENTOS6) -f Dockerfile-centos6 .

#
# Builds a Docker buildbox for CentOS 6 FIPS builds
#
.PHONY:buildbox-centos6-fips
buildbox-centos6-fips:
	docker build \
		--build-arg UID=$(UID) \
		--build-arg GID=$(GID) \
		--build-arg RUNTIME=$(RUNTIME) \
		--cache-from $(BUILDBOXCENTOS6FIPS) \
		--tag $(BUILDBOXCENTOS6FIPS) -f Dockerfile-centos6-fips .

#
# Builds a Docker container for building mkdocs documentation
#
.PHONY:
docsbox:
	docker build --build-arg UID=$$(id -u) \
		         --build-arg GID=$$(id -g) \
		         --build-arg USER=jenkins \
	             --build-arg WORKDIR=$(DOCSDIR) \
		         --tag $(DOCSBOX) -f docs.dockerfile .

#
# Removes the docker image
#
.PHONY:clean
clean:
	docker image rm --force $(BUILDBOX)
	docker image rm --force $(DOCSBOX)

#
# Runs tests inside a build container
#
.PHONY:test
test: buildbox
	docker run \
		--env TELEPORT_ETCD_TEST="yes" \
		$(DOCKERFLAGS) $(BCCFLAGS) $(NOROOT) -t $(BUILDBOX) \
		/bin/bash -c \
		"examples/etcd/start-etcd.sh & sleep 1; \
		ssh-agent > external.agent.tmp && source external.agent.tmp; \
		cd $(SRCDIR) && make TELEPORT_DEBUG=0 FLAGS='-cover -race' clean test"

.PHONY:integration
integration: buildbox
	docker run $(DOCKERFLAGS) $(BCCFLAGS) $(NOROOT) -t $(BUILDBOX) \
		/bin/bash -c "make -C $(SRCDIR) FLAGS='-cover' integration"

#
# Runs linters on new changes inside a build container.
#
.PHONY:lint
lint: buildbox
	docker run $(DOCKERFLAGS) $(BCCFLAGS) $(NOROOT) -t $(BUILDBOX) \
		/bin/bash -c "make -C $(SRCDIR) lint"

#
# Builds docs
#
.PHONY:docs
docs: docsbox
	docker run --rm=true -ti $(NOROOT) \
		-v $$(pwd)/..:$(DOCSDIR) \
		-v /tmp:/tmp \
		-w $(DOCSDIR) \
		-h $(DOCSHOST) $(DOCSBOX) \
		./docs/build.sh

#
# allows to enter into bash shell of docs container
#
.PHONY:
enter-docs: docsbox
	docker run --rm=true $(NOROOT) \
		-v $$(pwd)/..:$(DOCSDIR) \
		-v /tmp:/tmp \
		-w $(DOCSDIR) \
		-h $(DOCSHOST) \
		-ti $(DOCSBOX) /bin/bash

#
# Runs docs website on localhost
#
.PHONY:run-docs
run-docs: docsbox
	docker run -ti $(NOROOT) \
		-v $$(pwd)/..:$(DOCSDIR) \
		-v /tmp:/tmp \
		-e HOME=$(SRCDIR)/build.assets \
		-p 6600:6600 \
		-w $(DOCSDIR) \
		$(DOCSBOX) docs/run.sh

#
# Starts shell inside the build container
#
.PHONY:enter
enter: buildbox
	docker run $(DOCKERFLAGS) $(BCCFLAGS) -ti $(NOROOT) \
		-e HOME=$(SRCDIR)/build.assets -w $(SRCDIR) $(BUILDBOX) /bin/bash

#
# Create a Teleport package using the build container.
#
.PHONY:release
release: buildbox
	docker run $(DOCKERFLAGS) $(BCCFLAGS) -i $(NOROOT) $(BUILDBOX) \
		/usr/bin/make release -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(RUNTIME)

#
# Create a Teleport FIPS package using the build container.
# This is a special case because it only builds and packages the Enterprise FIPS binaries, no OSS.
#
.PHONY:release-fips
release-fips: buildbox-fips
	@if [ -z ${VERSION} ]; then echo "VERSION is not set"; exit 1; fi
	docker run $(DOCKERFLAGS) $(BCCFLAGS) -i $(NOROOT) $(BUILDBOXFIPS) \
		/usr/bin/make -C e release -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(RUNTIME) FIPS=yes VERSION=$(VERSION) GITTAG=v$(VERSION)

#
# Create a Teleport package for CentOS 6 using the build container.
#
.PHONY:release-centos6
release-centos6: buildbox-centos6
	docker run $(DOCKERFLAGS) -i $(NOROOT) $(BUILDBOXCENTOS6) \
		/usr/bin/make release -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(RUNTIME)

#
# Create a Teleport FIPS package for CentOS 6 using the build container.
# This is a special case because it only builds and packages the Enterprise FIPS binaries, no OSS.
#
.PHONY:release-centos6-fips
release-centos6-fips: buildbox-centos6-fips
	docker run $(DOCKERFLAGS) -i $(NOROOT) $(BUILDBOXCENTOS6FIPS) \
		/usr/bin/make release -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(RUNTIME) FIPS=yes

#
# Create a Windows Teleport package using the build container.
#
.PHONY:release-windows
release-windows: buildbox
	docker run $(DOCKERFLAGS) -i $(NOROOT) $(BUILDBOX) \
		/usr/bin/make release -e ADDFLAGS="$(ADDFLAGS)" OS=windows
