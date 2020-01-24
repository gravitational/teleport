#
# This Makefile is used for producing official Teleport releases
#
DOCSBOX=teleport-docsbox:latest
DOCSHOST=teleport-docs
DOCSDIR=/teleport

HOSTNAME=buildbox
SRCDIR=/gopath/src/github.com/gravitational/teleport
DOCKERFLAGS := --rm=true -v "$$(pwd)/../":$(SRCDIR) -v /tmp:/tmp -w $(SRCDIR) -h $(HOSTNAME)
BCCFLAGS := -v "$$(pwd)/bcc:/usr/include/bcc"
ADDFLAGS=-ldflags -w
NOROOT=-u $$(id -u):$$(id -g)
KUBECONFIG ?=
TEST_KUBE ?=

OS ?= linux
ARCH ?= amd64
RUNTIME ?= go1.13.2

BBOX=teleport-buildbox:$(RUNTIME)
BBOXFIPS=teleport-buildbox-fips:$(RUNTIME)
BBOXCENTOS6=teleport-buildbox-centos6:$(RUNTIME)
BBOXCENTOS6FIPS=teleport-buildbox-centos6-fips:$(RUNTIME)

ifneq ("$(KUBECONFIG)","")
DOCKERFLAGS := $(DOCKERFLAGS) -v $(KUBECONFIG):/mnt/kube/config -e KUBECONFIG=/mnt/kube/config -e TEST_KUBE=$(TEST_KUBE)
endif
export


#
# Build 'teleport' release inside a docker container
#
.PHONY:build
build: bbox
	docker run $(DOCKERFLAGS) $(BCCFLAGS) $(NOROOT) $(BBOX) \
		make -C $(SRCDIR) ADDFLAGS='$(ADDFLAGS)' release

#
# Build 'teleport' release inside a docker container
#
.PHONY:build-binaries
build-binaries: bbox
	docker run $(DOCKERFLAGS) $(BCCFLAGS) $(NOROOT) $(BBOX) \
		make -C $(SRCDIR) ADDFLAGS='$(ADDFLAGS)' all

#
# Builds a Docker container which is used for building official Teleport
# binaries and docs
#
.PHONY:bbox
bbox:
	docker build \
		--build-arg UID=$$(id -u) \
		--build-arg GID=$$(id -g) \
		--build-arg RUNTIME=$(RUNTIME) \
		--tag $(BBOX) .

#
# Builds a Docker buildbox for FIPS
#
.PHONY:bbox-fips
bbox-fips:
	docker build \
		--build-arg UID=$$(id -u) \
		--build-arg GID=$$(id -g) \
		--build-arg RUNTIME=$(RUNTIME) \
		--tag $(BBOXFIPS) -f Dockerfile-fips .

#
# Builds a Docker container for CentOS 6 builds 
#
.PHONY:bbox-centos6
bbox-centos6:
	docker build \
		--build-arg UID=$$(id -u) \
		--build-arg GID=$$(id -g) \
		--build-arg RUNTIME=$(RUNTIME) \
		--tag $(BBOXCENTOS6) -f Dockerfile-centos6 .

#
# Builds a Docker buildbox for CentOS 6 FIPS builds
#
.PHONY:bbox-centos6-fips
bbox-centos6-fips:
	docker build \
		--build-arg UID=$$(id -u) \
		--build-arg GID=$$(id -g) \
		--build-arg RUNTIME=$(RUNTIME) \
		--tag $(BBOXCENTOS6FIPS) -f Dockerfile-centos6-fips .

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
	docker image rm --force $(BBOX)
	docker image rm --force $(DOCSBOX)

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
enter: bbox
	docker run $(DOCKERFLAGS) $(BCCFLAGS) -ti $(NOROOT) \
		-e HOME=$(SRCDIR)/build.assets -w $(SRCDIR) $(BBOX) /bin/bash

#
# Create a Teleport package using the build container.
#
.PHONY:release
release: bbox
	docker run $(DOCKERFLAGS) $(BCCFLAGS) -i $(NOROOT) $(BBOX) \
		/usr/bin/make release -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(RUNTIME)

#
# Create a Teleport FIPS package using the build container.
# This is a special case because it only builds and packages the Enterprise FIPS binaries, no OSS.
#
.PHONY:release-fips
release-fips: bbox-fips
	@if [ -z ${VERSION} ]; then echo "VERSION is not set"; exit 1; fi
	docker run $(DOCKERFLAGS) $(BCCFLAGS) -i $(NOROOT) $(BBOXFIPS) \
		/usr/bin/make -C e release -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(RUNTIME) FIPS=yes VERSION=$(VERSION) GITTAG=v$(VERSION)

#
# Create a Teleport package for CentOS 6 using the build container.
#
.PHONY:release-centos6
release-centos6: bbox-centos6
	docker run $(DOCKERFLAGS) $(BCCFLAGS) -i $(NOROOT) $(BBOXCENTOS6) \
		/usr/bin/make release -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(RUNTIME)

#
# Create a Teleport FIPS package using the build container.
# This is a special case because it only builds and packages the Enterprise FIPS binaries, no OSS.
#
.PHONY:release-centos6-fips
release-centos6-fips: bbox-centos6-fips
	docker run $(DOCKERFLAGS) $(BCCFLAGS) -i $(NOROOT) $(BBOXCENTOS6FIPS) \
		/usr/bin/make release -e ADDFLAGS="$(ADDFLAGS)" OS=$(OS) ARCH=$(ARCH) RUNTIME=$(RUNTIME) FIPS=yes

#
# Create a Windows Teleport package using the build container.
#
.PHONY:release-windows
release-windows: bbox
	docker run $(DOCKERFLAGS) -i $(NOROOT) $(BBOX) \
		/usr/bin/make release -e ADDFLAGS="$(ADDFLAGS)" OS=windows
