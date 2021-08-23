# This makefile usually runs as a sub-make by ../Makefile
# But running regular 'make' and 'make clean' works.
ifneq ("$(wildcard /bin/bash)","")
SHELL := /bin/bash -o pipefail
endif
BUILDDIR ?= build
LOCALDIR := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))
WEBASSETS_BUILDDIR ?= $(LOCALDIR)../lib/web/build
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s $(LDFLAGS)'
CGOFLAG ?= CGO_ENABLED=1
RELEASE_ENT = teleport-ent-$(GITTAG)-$(OS)-$(ARCH)-bin
BINARIES ?= $(BUILDDIR)/teleport $(BUILDDIR)/tctl $(BUILDDIR)/tsh
HOUSTON_DIR := "vendor/github.com/gravitational/houston/"

ifeq ("$(OS)","linux")
# ARM builds need to specify the correct C compiler
ifeq ("$(ARCH)","arm")
CGOFLAG = CGO_ENABLED=1 CC=arm-linux-gnueabihf-gcc
endif
# ARM64 builds need to specify the correct C compiler
ifeq ("$(ARCH)","arm64")
CGOFLAG = CGO_ENABLED=1 CC=aarch64-linux-gnu-gcc
endif
endif

DOCKER_IMAGE_ENT ?= quay.io/gravitational/teleport-ent
DOCKER_IMAGE_ENT_CI ?= quay.io/gravitational/teleport-ent-ci

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)

FIPS_ENT_MESSAGE := "without FIPS support"
# FIPS support must be requested at build time.
ifneq ("$(FIPS)","")
FIPS_TAG := fips
FIPS_ENT_MESSAGE := "with FIPS support"
RELEASE_ENT = teleport-ent-$(GITTAG)-$(OS)-$(ARCH)-fips-bin
# See the comment in tool/modules/fips.go.hide for explanation.
$(shell ln -sf fips.go.hide tool/modules/fips.go)
else
# See the comment in tool/modules/fips.go.hide for explanation.
$(shell rm tool/modules/fips.go)
endif

PAM_ENT_MESSAGE := "without PAM support"
# PAM support will only be built into Teleport if headers exist at build time.
ifneq ("$(wildcard /usr/include/security/pam_appl.h)","")
PAM_TAG = pam
PAM_ENT_MESSAGE := "with PAM support"
endif

BPF_ENT_MESSAGE := "without BPF support"
# We don't compile BPF for anything except regular non-FIPS linux/amd64 for now, as other builds
# have compilation issues that require fixing.
with_bpf := no
ifeq ("$(OS)","linux")
ifeq ("$(ARCH)","amd64")
ifneq ("$(wildcard /usr/include/bpf/libbpf.h)","")
with_bpf := yes
BPF_TAG := bpf
BPF_ENT_MESSAGE := "with BPF support"
INCLUDES :=

CGOFLAG = CGO_ENABLED=1 CGO_LDFLAGS="-Wl,-Bstatic -lbpf -lelf -lz -Wl,-Bdynamic"
endif
endif
endif

RELEASE_ENT_MESSAGE := "Building Enterprise with GOOS=$(OS) GOARCH=$(ARCH) and $(PAM_ENT_MESSAGE) and $(FIPS_ENT_MESSAGE) and $(BPF_ENT_MESSAGE)."

# All build Enterprise version of "teleport" and "tctl" binaries and copy
# over tsh from OSS.
.PHONY: all
all:
	@echo "---> $(RELEASE_ENT_MESSAGE)"
	$(MAKE) $(BINARIES)

# By making these 3 targets below (tsh, tctl and teleport) PHONY we are solving
# several problems:
# * Build will rely on go build internal caching https://golang.org/doc/go1.10 at all times
# * Manual change detection was broken on a large dependency tree
# If you are considering changing this behavior, please consult with dev team first
.PHONY: $(BUILDDIR)/tctl
$(BUILDDIR)/tctl:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" -o $(BUILDDIR)/tctl $(BUILDFLAGS) ./tool/tctl

.PHONY: $(BUILDDIR)/teleport
$(BUILDDIR)/teleport: bpf-bytecode-e ensure-webassets-e teleport-fips-check
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG) $(WEBASSETS_TAG)" -o $(BUILDDIR)/teleport $(BUILDFLAGS) ./tool/teleport

.PHONY: $(BUILDDIR)/tsh
$(BUILDDIR)/tsh:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "$(PAM_TAG) $(FIPS_TAG)" -o $(BUILDDIR)/tsh $(BUILDFLAGS) ../tool/tsh

#
# BPF support (IF ENABLED)
#
ifeq ("$(with_bpf)","yes")
.PHONY: bpf-bytecode-e
bpf-bytecode-e:
	$(MAKE) -C ../ bpf-bytecode

else
.PHONY: bpf-bytecode-e
bpf-bytecode-e:
endif

# If FIPS mode was requested, first compile an extra Teleport binary with symbols included and check that it contains BoringCrypto references.
.PHONY: teleport-fips-check
teleport-fips-check:
	@if [ "$(FIPS)" != "" ]; then \
		echo "FIPS mode requested, pre-compiling and checking binary for BoringCrypto symbols."; \
		GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG) $(WEBASSETS_TAG)" -o $(BUILDDIR)/teleport-unstripped -ldflags "" ./tool/teleport; \
		if ! go tool nm $(BUILDDIR)/teleport-unstripped 2>&1 | grep -q boringcrypto; then \
			echo "-- FIPS mode was requested, but the compiled teleport binary does not contain any BoringCrypto symbols. This is an error!"; \
			echo "-- BoringCrypto is currently only supported for linux/amd64 as per https://go.googlesource.com/go/+/dev.boringcrypto/misc/boring#caveat"; \
			exit 1; \
		fi; \
		rm -f $(BUILDDIR)/teleport-unstripped; \
	fi

# make clean: cleans the build
.PHONY: clean
clean:
	@echo "---> Cleaning up Enterprise build artifacts."
	rm -rf $(BUILDDIR)
	rm -rf *gz
	-rm build_houston
	-rm start_houston

# make release - Produces a binary release tarball.
.PHONY:
export
release:
	@echo
	@echo "---> $(RELEASE_ENT_MESSAGE)"
	$(MAKE) --no-print-directory release-unix

# These are aliases used to make build commands uniform.
.PHONY: release-amd64
release-amd64:
	$(MAKE) release ARCH=amd64

.PHONY: release-amd64-fips
release-amd64-fips:
	$(MAKE) release ARCH=amd64 FIPS=yes

.PHONY: release-386
release-386:
	$(MAKE) release ARCH=386

.PHONY: release-arm
release-arm:
	$(MAKE) release ARCH=arm

.PHONY: release-arm64
release-arm64:
	$(MAKE) release ARCH=arm64

# make release-unix - Produces a binary release tarball containing teleport,
# tctl, and tsh.
.PHONY:
release-unix: clean full
	@echo "---> Creating Enterprise release archive."
	cp ../README.md $(BUILDDIR)/
	cp ../CHANGELOG.md $(BUILDDIR)/
	cp ../build.assets/install $(BUILDDIR)/
	echo $(GITTAG) > $(BUILDDIR)/VERSION
	cp -rf $(BUILDDIR) teleport-ent
	cp -rf ../examples teleport-ent/
	tar --sort=name --owner=root:0 --group=root:0 --mtime='UTC 2015-03-02' --format=gnu -c teleport-ent | gzip -n > $(RELEASE_ENT).tar.gz
	rm -rf teleport-ent
	@echo "---> Created $(RELEASE_ENT).tar.gz."

# make full - produces a teleport binary with the web assets built-in
.PHONY:full
full: check-build-vars $(WEBASSETS_BUILDDIR)/webassets
	@echo "---> Attaching Enterprise web assets."
	$(MAKE) all WEBASSETS_TAG="webassets_embed"

# build/webassets.zip archive contains the web assets (UI) which gets
# embedded into teleport binary
.PHONY: $(WEBASSETS_BUILDDIR)/webassets
$(WEBASSETS_BUILDDIR)/webassets: $(WEBASSETS_BUILDDIR)
	@echo "---> Copying Enterprise web assets."
	mkdir -p $(WEBASSETS_BUILDDIR)/webassets
	cd ../webassets/e/teleport/ ; cp -r . $@

.PHONY: $(WEBASSETS_BUILDDIR)
$(WEBASSETS_BUILDDIR):
	rm -fr $@
	mkdir -p $@

.PHONY: check-build-vars
check-build-vars:
ifndef OS
	$(error OS is not set)
endif
ifndef ARCH
	$(error ARCH is not set)
endif
ifndef GITTAG
	$(error GITTAG is not set)
endif

#
# test runs teleport enterprise tests
#
.PHONY: test
test:
	go test -v ./lib/... && \
	go vet ./lib/...

.PHONY: integration
integration: start-houston
	go test -v ./integration/... -check.v
	$(MAKE) stop-houston

build-houston:
	sudo $(MAKE) -C $(HOUSTON_DIR) image
	@touch build-houston

start-houston: build-houston
	@docker run -d --rm=true \
		-e HOUSTON_API_HOST="localhost" \
		-e HOUSTON_WEB_HOST="web.localhost" \
		-e HOUSTON_STRIPE_SECRET_KEY="$(HOUSTON_STRIPE_SECRET_KEY)" \
		-e HOUSTON_STRIPE_PUBLIC_KEY="$(HOUSTON_STRIPE_PUBLIC_KEY)" \
		-e HOUSTON_AUTH0_CLIENT_ID="$(HOUSTON_AUTH0_CLIENT_ID)" \
		-e HOUSTON_AUTH0_CLIENT_SECRET="$(HOUSTON_AUTH0_CLIENT_SECRET)" \
		-e HOUSTON_AUTH0_AUDIENCE="$(HOUSTON_AUTH0_AUDIENCE)" \
		-e HOUSTON_AUTH0_ISSUER="$(HOUSTON_AUTH0_ISSUER)" \
		-e HOUSTON_AUTH0_AUTH_API_URL="$(HOUSTON_AUTH0_AUTH_API_URL)" \
		-e HOUSTON_AUTH0_MGMT_API_URL="$(HOUSTON_AUTH0_MGMT_API_URL)" \
		-e HOUSTON_AUTH0_REDIRECT_URL="$(HOUSTON_AUTH0_REDIRECT_URL)" \
		-e HOUSTON_ZENDESK_API_USER="$(HOUSTON_ZENDESK_API_USER)" \
		-e HOUSTON_ZENDESK_API_TOKEN="$(HOUSTON_ZENDESK_API_TOKEN)" \
		-e HOUSTON_ZENDESK_API_URL="$(HOUSTON_ZENDESK_API_URL)" \
		-e HOUSTON_GITHUB_USERNAME="$(HOUSTON_GITHUB_USERNAME)" \
		-e HOUSTON_GITHUB_ACCESS_TOKEN="$(HOUSTON_GITHUB_ACCESS_TOKEN)" \
		-e HOUSTON_SALESFORCE_CLIENT_ID="$(HOUSTON_SALESFORCE_CLIENT_ID)" \
		-e HOUSTON_SALESFORCE_CLIENT_SECRET="$(HOUSTON_SALESFORCE_CLIENT_SECRET)" \
		-e HOUSTON_SALESFORCE_USERNAME="$(HOUSTON_SALESFORCE_USERNAME)" \
		-e HOUSTON_SALESFORCE_PASSWORD="$(HOUSTON_SALESFORCE_PASSWORD)" \
		-p 10000:10000 \
	       	houston:0.0.1 /opt/houston/houston
	@touch start-houston

.PHONY: stop-houston
stop-houston:
	docker stop $$(docker ps -q --filter ancestor=houston:0.0.1)
	@rm start-houston

# Dockerized build: useful for making Linux binaries on OSX
.PHONY:docker-binaries
docker-binaries: clean
	make -C ../build.assets build-enterprise-binaries

.PHONY: image
image: clean docker-binaries
	cp ../build.assets/charts/Dockerfile $(BUILDDIR)/
	cd $(BUILDDIR) && docker build --no-cache . -t $(DOCKER_IMAGE_ENT):$(VERSION)

.PHONY:docker-binaries-fips
docker-binaries-fips: clean
	@if [ -z ${VERSION} ]; then echo "VERSION is not set"; exit 1; fi
	make -C ../build.assets build-binaries-fips

.PHONY: image-fips
image-fips: docker-binaries-fips
	@if [ -z ${VERSION} ]; then echo "VERSION is not set"; exit 1; fi
	cp ../build.assets/charts/Dockerfile-fips $(BUILDDIR)/Dockerfile
	cd $(BUILDDIR) && docker build --no-cache . -t $(DOCKER_IMAGE_ENT):$(VERSION)-fips

.PHONY: publish
publish:
	docker push $(DOCKER_IMAGE_ENT):$(VERSION)

.PHONY: publish-fips
publish-fips:
	docker push $(DOCKER_IMAGE_ENT):$(VERSION)-fips

# Docker image build in CI.
# This is run to build and push Docker images to a private repository as part of the build process.
# When we are ready to make the images public after testing (i.e. when publishing a release), we pull these
# images down, retag them and push them up to the production repo so they're available for use.
# This job can be removed/consolidated after we switch over completely from using Jenkins to using Drone.
.PHONY: image-ci
image-ci: clean docker-binaries
	@if [ -z ${VERSION} ]; then echo "VERSION is not set"; exit 1; fi
	cp ../build.assets/charts/Dockerfile $(BUILDDIR)/
	cd $(BUILDDIR) && docker build --no-cache . -t $(DOCKER_IMAGE_ENT_CI):$(VERSION)

.PHONY: image-fips-ci
image-fips-ci: docker-binaries-fips
	@if [ -z ${VERSION} ]; then echo "VERSION is not set"; exit 1; fi
	cp ../build.assets/charts/Dockerfile-fips $(BUILDDIR)/Dockerfile
	cd $(BUILDDIR) && docker build --no-cache . -t $(DOCKER_IMAGE_ENT_CI):$(VERSION)-fips

.PHONY: publish-ci
publish-ci:
	docker push $(DOCKER_IMAGE_ENT_CI):$(VERSION)

.PHONY: publish-fips-ci
publish-fips-ci:
	docker push $(DOCKER_IMAGE_ENT_CI):$(VERSION)-fips

# Build chart
.PHONY: chart
chart: VERSION := $(shell $(MAKE) -s -C .. print-version)
chart:
	rm -f teleport-$(VERSION).tgz
	helm package ../examples/chart/teleport --version=$(VERSION)

# install-chart
.PHONY:
install-chart: VERSION := $(shell $(MAKE) -s -C .. print-version)
install-chart: chart
	helm init
	helm install teleport-$(VERSION).tgz -n teleport

# debug-chart
.PHONY:
debug-chart: VERSION := $(shell $(MAKE) -s -C .. print-version)
debug-chart:
	helm init
	helm install --dry-run --debug teleport-$(VERSION).tgz -n teleport

# uninstall-chart
.PHONY:
uninstall-chart:
	helm del --purge teleport

# upgrade chart
.PHONY:
upgrade-chart: VERSION := $(shell $(MAKE) -s -C .. print-version)
upgrade-chart: chart
	helm upgrade teleport teleport-$(VERSION).tgz

RUNTIME_SECTION ?=
TARBALL_PATH_SECTION ?=

ifneq ("$(RUNTIME)", "")
	RUNTIME_SECTION := -r $(RUNTIME)
endif
ifneq ("$(ENT_TARBALL_PATH)", "")
	TARBALL_PATH_SECTION := -s $(ENT_TARBALL_PATH)
endif

# build .pkg
.PHONY: pkg
pkg:
	mkdir -p $(BUILDDIR)/
	cp ../build.assets/build-package.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	# arch and runtime are currently ignored on OS X
	# we pass them through for consistency - they will be dropped by the build script
	cd $(BUILDDIR) && ./build-package.sh -t ent -v $(VERSION) -p pkg -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)

# build .rpm
.PHONY: rpm
rpm:
	mkdir -p $(BUILDDIR)/
	cp ../build.assets/build-package.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	cp -a ../build.assets/rpm-sign $(BUILDDIR)/
	cd $(BUILDDIR) && ./build-package.sh -t ent -v $(VERSION) -p rpm -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)

# build unsigned .rpm (for testing)
.PHONY: rpm-unsigned
rpm-unsigned:
	$(MAKE) UNSIGNED_RPM=true rpm

# build .deb
.PHONY: deb
deb:
	mkdir -p $(BUILDDIR)/
	cp ../build.assets/build-package.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	cd $(BUILDDIR) && ./build-package.sh -t ent -v $(VERSION) -p deb -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)

.PHONY: ensure-webassets-e
ensure-webassets-e:
	$(MAKE) -s -C .. ensure-webassets-e

