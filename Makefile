# Make targets:
#
#  all    : builds all binaries in development mode, without web assets (default)
#  full   : builds all binaries for PRODUCTION use
#  release: prepares a release tarball
#  clean  : removes all buld artifacts
#  test   : runs tests

# To update the Teleport version, update VERSION variable:
# Naming convention:
#   Stable releases:   "1.0.0"
#   Pre-releases:      "1.0.0-alpha.1", "1.0.0-beta.2", "1.0.0-rc.3"
#   Master/dev branch: "1.0.0-dev"
VERSION=7.0.0-dev

DOCKER_IMAGE ?= quay.io/gravitational/teleport
DOCKER_IMAGE_CI ?= quay.io/gravitational/teleport-ci

GO_BINARY ?= go

# These are standard autotools variables, don't change them please
BUILDDIR ?= build
ASSETS_BUILDDIR ?= lib/web/build
BINDIR ?= /usr/local/bin
DATADIR ?= /usr/local/share/teleport
ADDFLAGS ?=
PWD ?= `pwd`
GOPKGDIR ?= `$(GO_BINARY) env GOPATH`/pkg/`$(GO_BINARY) env GOHOSTOS`_`$(GO_BINARY) env GOARCH`/github.com/gravitational/teleport*
TELEPORT_DEBUG ?= no
GITTAG=v$(VERSION)
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s'
CGOFLAG ?= CGO_ENABLED=1
# Windows requires extra parameters to cross-compile with CGO.
ifeq ("$(OS)","windows")
BUILDFLAGS = $(ADDFLAGS) -ldflags '-w -s' -buildmode=exe
CGOFLAG = CGO_ENABLED=1 CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++
endif

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

OS ?= $(shell $(GO_BINARY) env GOOS)
ARCH ?= $(shell $(GO_BINARY) env GOARCH)
FIPS ?=
RELEASE = teleport-$(GITTAG)-$(OS)-$(ARCH)-bin

# FIPS support must be requested at build time.
FIPS_MESSAGE := "without FIPS support"
ifneq ("$(FIPS)","")
FIPS_TAG := fips
FIPS_MESSAGE := "with FIPS support"
RELEASE = teleport-$(GITTAG)-$(OS)-$(ARCH)-fips-bin
endif

# PAM support will only be built into Teleport if headers exist at build time.
PAM_MESSAGE := "without PAM support"
ifneq ("$(wildcard /usr/include/security/pam_appl.h)","")
PAM_TAG := pam
PAM_MESSAGE := "with PAM support"
else
# PAM headers for Darwin live under /usr/local/include/security instead, as SIP
# prevents us from modifying/creating /usr/include/security on newer versions of MacOS
ifneq ("$(wildcard /usr/local/include/security/pam_appl.h)","")
PAM_TAG := pam
PAM_MESSAGE := "with PAM support"
endif
endif

# BPF support will only be built into Teleport if headers exist at build time.
BPF_MESSAGE := "without BPF support"

# BPF cannot currently be compiled on ARMv7 due to this bug: https://github.com/iovisor/gobpf/issues/272
# ARM64 builds are not affected.
ifneq ("$(ARCH)","arm")
ifneq ("$(wildcard /usr/include/bcc/libbpf.h)","")
BPF_TAG := bpf
BPF_MESSAGE := "with BPF support"
endif
endif

# On Windows only build tsh. On all other platforms build teleport, tctl,
# and tsh.
BINARIES=$(BUILDDIR)/teleport $(BUILDDIR)/tctl $(BUILDDIR)/tsh
RELEASE_MESSAGE := "Building with GOOS=$(OS) GOARCH=$(ARCH) and $(PAM_MESSAGE) and $(FIPS_MESSAGE) and $(BPF_MESSAGE)."
ifeq ("$(OS)","windows")
BINARIES=$(BUILDDIR)/tsh
endif

VERSRC = version.go gitref.go

KUBECONFIG ?=
TEST_KUBE ?=
export

#
# 'make all' builds all 3 executables and places them in the current directory.
#
# IMPORTANT: the binaries will not contain the web UI assets and `teleport`
#            won't start without setting the environment variable DEBUG=1
#            This is the default build target for convenience of working on
#            a web UI.
.PHONY: all
all: $(VERSRC)
	@echo "---> Building OSS binaries."
	$(MAKE) $(BINARIES)

# By making these 3 targets below (tsh, tctl and teleport) PHONY we are solving
# several problems:
# * Build will rely on go build internal caching https://golang.org/doc/go1.10 at all times
# * Manual change detection was broken on a large dependency tree
# If you are considering changing this behavior, please consult with dev team first
.PHONY: $(BUILDDIR)/tctl
$(BUILDDIR)/tctl:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) $(GO_BINARY) build -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" -o $(BUILDDIR)/tctl $(BUILDFLAGS) ./tool/tctl

.PHONY: $(BUILDDIR)/teleport
$(BUILDDIR)/teleport: ensure-webassets
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) $(GO_BINARY) build -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG) $(WEBASSETS_TAG)" -o $(BUILDDIR)/teleport $(BUILDFLAGS) ./tool/teleport

.PHONY: $(BUILDDIR)/tsh
$(BUILDDIR)/tsh:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) $(GO_BINARY) build -tags "$(PAM_TAG) $(FIPS_TAG)" -o $(BUILDDIR)/tsh $(BUILDFLAGS) ./tool/tsh

#
# make full - Builds Teleport binaries with the built-in web assets and
# places them into $(BUILDDIR). On Windows, this target is skipped because
# only tsh is built.
#
.PHONY:full
full: $(ASSETS_BUILDDIR)/webassets.zip
ifneq ("$(OS)", "windows")
	$(MAKE) all WEBASSETS_TAG="webassets_embed"
endif

#
# make full-ent - Builds Teleport enterprise binaries
#
.PHONY:full-ent
full-ent:
ifneq ("$(OS)", "windows")
	@if [ -f e/Makefile ]; then $(MAKE) -C e full; fi
endif

#
# make clean - Removed all build artifacts.
#
.PHONY: clean
clean:
	@echo "---> Cleaning up OSS build artifacts."
	rm -rf $(BUILDDIR)
	-$(GO_BINARY) clean -cache
	rm -rf $(GOPKGDIR)
	rm -rf teleport
	rm -rf *.gz
	rm -rf *.zip
	rm -f gitref.go

#
# make release - Produces a binary release tarball.
#
.PHONY:
export
release:
	@echo "---> $(RELEASE_MESSAGE)"
ifeq ("$(OS)", "windows")
	$(MAKE) --no-print-directory release-windows
else
	$(MAKE) --no-print-directory release-unix
endif

# These are aliases used to make build commands uniform.
.PHONY: release-amd64
release-amd64:
	$(MAKE) release ARCH=amd64

.PHONY: release-386
release-386:
	$(MAKE) release ARCH=386

.PHONY: release-arm
release-arm:
	$(MAKE) release ARCH=arm

.PHONY: release-arm64
release-arm64:
	$(MAKE) release ARCH=arm64

#
# make release-unix - Produces a binary release tarball containing teleport,
# tctl, and tsh.
#
.PHONY:
release-unix: clean full
	@echo "---> Creating OSS release archive."
	mkdir teleport
	cp -rf $(BUILDDIR)/* \
		examples \
		build.assets/install\
		README.md \
		CHANGELOG.md \
		teleport/
	echo $(GITTAG) > teleport/VERSION
	tar -czf $(RELEASE).tar.gz teleport
	rm -rf teleport
	@echo "---> Created $(RELEASE).tar.gz."
	@if [ -f e/Makefile ]; then \
		rm -fr $(WEBASSETS_BUILDDIR)/webassets.zip; \
		$(MAKE) -C e release; \
	fi

#
# make release-windows - Produces a binary release tarball containing teleport,
# tctl, and tsh.
#
.PHONY:
release-windows: clean all
	@echo "---> Creating OSS release archive."
	mkdir teleport
	cp -rf $(BUILDDIR)/* \
		README.md \
		CHANGELOG.md \
		teleport/
	mv teleport/tsh teleport/tsh.exe
	echo $(GITTAG) > teleport/VERSION
	zip -9 -y -r -q $(RELEASE).zip teleport/
	rm -rf teleport/
	@echo "---> Created $(RELEASE).zip."

#
# Remove trailing whitespace in all markdown files under docs/.
#
# Note: this runs in a busybox container to avoid incompatibilities between
# linux and macos CLI tools.
#
.PHONY:docs-fix-whitespace
docs-fix-whitespace:
	docker run --rm -v $(PWD):/teleport busybox \
		find /teleport/docs/ -type f -name '*.md' -exec sed -E -i 's/\s+$$//g' '{}' \;

#
# Test docs for trailing whitespace and broken links
#
.PHONY:docs-test
docs-test: docs-test-whitespace

#
# Check for trailing whitespace in all markdown files under docs/
#
.PHONY:docs-test-whitespace
docs-test-whitespace:
	if find docs/ -type f -name '*.md' | xargs grep -E '\s+$$'; then \
		echo "trailing whitespace found in docs/ (see above)"; \
		echo "run 'make docs-fix-whitespace' to fix it"; \
		exit 1; \
	fi


#
# Runs all Go/shell tests, called by CI/CD.
#
.PHONY: test
test: test-sh test-api test-go

#
# Runs all Go tests except integration, called by CI/CD.
# Chaos tests have high concurrency, run without race detector and have TestChaos prefix.
#
.PHONY: test-go
test-go: ensure-webassets
test-go: FLAGS ?= '-race'
test-go: PACKAGES := $(shell $(GO_BINARY) list ./... | grep -v integration)
test-go: CHAOS_FOLDERS := $(shell find . -type f -name '*chaos*.go' -not -path '*/vendor/*' | xargs dirname | uniq)
test-go: $(VERSRC)
	$(GO_BINARY) test -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" $(PACKAGES) $(FLAGS) $(ADDFLAGS)
	$(GO_BINARY) test -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" -test.run=TestChaos $(CHAOS_FOLDERS) -cover

# Runs API Go tests. These have to be run separately as the package name is different.
#
.PHONY: test-api
test-api:
test-api: FLAGS ?= '-race'
test-api: PACKAGES := $(shell cd api && $(GO_BINARY) list ./...)
test-api: $(VERSRC)
	GOMODCACHE=/tmp $(GO_BINARY) test -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" $(PACKAGES) $(FLAGS) $(ADDFLAGS)

# Find and run all shell script unit tests (using https://github.com/bats-core/bats-core)
.PHONY: test-sh
test-sh:
	@find . -iname "*.bats" -exec dirname {} \; | uniq | xargs -t -L1 bats $(BATSFLAGS)

#
# Integration tests. Need a TTY to work.
# Any tests which need to run as root must be skipped during regular integration testing.
#
.PHONY: integration
integration: FLAGS ?= -v -race
integration: PACKAGES := $(shell $(GO_BINARY) list ./... | grep integration)
integration:
	@echo KUBECONFIG is: $(KUBECONFIG), TEST_KUBE: $(TEST_KUBE)
	$(GO_BINARY) test -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" $(PACKAGES) $(FLAGS)

#
# Integration tests which need to be run as root in order to complete successfully
# are run separately to all other integration tests. Need a TTY to work.
#
INTEGRATION_ROOT_REGEX := ^TestRoot
.PHONY: integration-root
integration-root: FLAGS ?= -v -race
integration-root: PACKAGES := $(shell $(GO_BINARY) list ./... | grep integration)
integration-root:
	$(GO_BINARY) test -run "$(INTEGRATION_ROOT_REGEX)" $(PACKAGES) $(FLAGS)

#
# Lint the Go code.
# By default lint scans the entire repo. Pass GO_LINT_FLAGS='--new' to only scan local
# changes (or last commit).
#
.PHONY: lint
lint: lint-sh lint-helm lint-api lint-go

.PHONY: lint-go
lint-go: GO_LINT_FLAGS ?=
lint-go:
	golangci-lint run -c .golangci.yml $(GO_LINT_FLAGS)

# api is no longer part of the teleport package, so golangci-lint skips it by default
# GOMODCACHE needs to be set here as api downloads dependencies and cannot write to /go/pkg/mod/cache
.PHONY: lint-api
lint-api: GO_LINT_API_FLAGS ?=
lint-api:
	cd api && GOMODCACHE=/tmp golangci-lint run -c ../.golangci.yml $(GO_LINT_API_FLAGS)

# TODO(awly): remove the `--exclude` flag after cleaning up existing scripts
.PHONY: lint-sh
lint-sh: SH_LINT_FLAGS ?=
lint-sh:
	find . -type f -name '*.sh' | grep -v vendor | xargs \
		shellcheck \
		--exclude=SC2086 \
		$(SH_LINT_FLAGS)

	# lint AWS AMI scripts
	# SC1091 prints errors when "source" directives are not followed
	find assets/aws/files/bin -type f | xargs \
		shellcheck \
		--exclude=SC2086 \
		--exclude=SC1091 \
		--exclude=SC2129 \
		$(SH_LINT_FLAGS)

# Lints all the Helm charts found in directories under examples/chart and exits on failure
# If there is a .lint directory inside, the chart gets linted once for each .yaml file in that directory
.PHONY: lint-helm
lint-helm:
	for CHART in $$(find examples/chart -mindepth 1 -maxdepth 1 -type d); do \
		if [ -d $$CHART/.lint ]; then \
			for VALUES in $$CHART/.lint/*.yaml; do \
				echo "$$CHART: $$VALUES"; \
				helm lint --strict $$CHART -f $$VALUES || exit 1; \
				helm template test $$CHART -f $$VALUES 1>/dev/null || exit 1; \
			done \
		else \
			helm lint --strict $$CHART || exit 1; \
			helm template test $$CHART 1>/dev/null || exit 1; \
		fi \
	done

# This rule triggers re-generation of version.go and gitref.go if Makefile changes
$(VERSRC): Makefile
	VERSION=$(VERSION) $(MAKE) -f version.mk setver

# make tag - prints a tag to use with git for the current version
# 	To put a new release on Github:
# 		- bump VERSION variable
# 		- run make setver
# 		- commit changes to git
# 		- build binaries with 'make release'
# 		- run `make tag` and use its output to 'git tag' and 'git push --tags'
.PHONY: tag
tag:
	@echo "Run this:\n> git tag $(GITTAG)\n> git push --tags"


# build/webassets.zip archive contains the web assets (UI) which gets
# appended to teleport binary
$(ASSETS_BUILDDIR)/webassets.zip: | $(ASSETS_BUILDDIR)
ifneq ("$(OS)", "windows")
	@echo "---> Building OSS web assets."
	cd webassets/teleport/ ; zip -qr ../../$@ .
endif

$(ASSETS_BUILDDIR):
	mkdir -p $@

.PHONY: test-package
test-package: remove-temp-files
	$(GO_BINARY) test -v ./$(p)

.PHONY: test-grep-package
test-grep-package: remove-temp-files
	$(GO_BINARY) test -v ./$(p) -check.f=$(e)

.PHONY: cover-package
cover-package: remove-temp-files
	$(GO_BINARY) test -v ./$(p)  -coverprofile=/tmp/coverage.out
	$(GO_BINARY) tool cover -html=/tmp/coverage.out

.PHONY: profile
profile:
	$(GO_BINARY) tool pprof http://localhost:6060/debug/pprof/profile

.PHONY: sloccount
sloccount:
	find . -path ./vendor -prune -o -name "*.go" -print0 | xargs -0 wc -l

.PHONY: remove-temp-files
remove-temp-files:
	find . -name flymake_* -delete

# Dockerized build: useful for making Linux releases on OSX
.PHONY:docker
docker:
	make -C build.assets build

# Dockerized build: useful for making Linux binaries on OSX
.PHONY:docker-binaries
docker-binaries: clean
	make -C build.assets build-binaries

# Interactively enters a Docker container (which you can build and run Teleport inside of)
.PHONY:enter
enter:
	make -C build.assets enter

# grpc generates GRPC stubs from service definitions
.PHONY: grpc
grpc:
	make -C build.assets grpc

# buildbox-grpc generates GRPC stubs inside buildbox
.PHONY: buildbox-grpc
buildbox-grpc:
# standard GRPC output
	echo $$PROTO_INCLUDE
	find lib/ -iname *.proto | xargs clang-format -i -style='{ColumnLimit: 100, IndentWidth: 4, Language: Proto}'
	find api/ -iname *.proto | xargs clang-format -i -style='{ColumnLimit: 100, IndentWidth: 4, Language: Proto}'

	protoc -I=.:$$PROTO_INCLUDE \
		--proto_path=api/types/events \
		--gogofast_out=plugins=grpc:api/types/events \
		events.proto

	protoc -I=.:$$PROTO_INCLUDE \
		--proto_path=api/types/wrappers \
		--gogofast_out=plugins=grpc:api/types/wrappers \
		wrappers.proto

	protoc -I=.:$$PROTO_INCLUDE \
		--proto_path=api/types \
		--gogofast_out=plugins=grpc:api/types \
		types.proto

	protoc -I=.:$$PROTO_INCLUDE \
		--proto_path=api/client/proto \
		--gogofast_out=plugins=grpc:api/client/proto \
		authservice.proto

	cd lib/multiplexer/test && protoc -I=.:$$PROTO_INCLUDE \
	  --gogofast_out=plugins=grpc:.\
    *.proto

	cd lib/web && protoc -I=.:$$PROTO_INCLUDE \
	  --gogofast_out=plugins=grpc:.\
    *.proto

.PHONY: goinstall
goinstall:
	$(GO_BINARY) install $(BUILDFLAGS) \
		github.com/gravitational/teleport/tool/tsh \
		github.com/gravitational/teleport/tool/teleport \
		github.com/gravitational/teleport/tool/tctl

# make install will installs system-wide teleport
.PHONY: install
install: build
	@echo "\n** Make sure to run 'make install' as root! **\n"
	cp -f $(BUILDDIR)/tctl      $(BINDIR)/
	cp -f $(BUILDDIR)/tsh       $(BINDIR)/
	cp -f $(BUILDDIR)/teleport  $(BINDIR)/
	mkdir -p $(DATADIR)


# Docker image build. Always build the binaries themselves within docker (see
# the "docker" rule) to avoid dependencies on the host libc version.
.PHONY: image
image: clean docker-binaries
	cp ./build.assets/charts/Dockerfile $(BUILDDIR)/
	cd $(BUILDDIR) && docker build --no-cache . -t $(DOCKER_IMAGE):$(VERSION)
	if [ -f e/Makefile ]; then $(MAKE) -C e image; fi

.PHONY: publish
publish: image
	docker push $(DOCKER_IMAGE):$(VERSION)
	if [ -f e/Makefile ]; then $(MAKE) -C e publish; fi

# Docker image build in CI.
# This is run to build and push Docker images to a private repository as part of the build process.
# When we are ready to make the images public after testing (i.e. when publishing a release), we pull these
# images down, retag them and push them up to the production repo so they're available for use.
# This job can be removed/consolidated after we switch over completely from using Jenkins to using Drone.
.PHONY: image-ci
image-ci: clean docker-binaries
	cp ./build.assets/charts/Dockerfile $(BUILDDIR)/
	cd $(BUILDDIR) && docker build --no-cache . -t $(DOCKER_IMAGE_CI):$(VERSION)
	if [ -f e/Makefile ]; then $(MAKE) -C e image-ci; fi

.PHONY: publish-ci
publish-ci: image-ci
	docker push $(DOCKER_IMAGE_CI):$(VERSION)
	if [ -f e/Makefile ]; then $(MAKE) -C e publish-ci; fi

.PHONY: print-version
print-version:
	@echo $(VERSION)

.PHONY: chart-ent
chart-ent:
	$(MAKE) -C e chart

RUNTIME_SECTION ?=
TARBALL_PATH_SECTION ?=

ifneq ("$(RUNTIME)", "")
	RUNTIME_SECTION := -r $(RUNTIME)
endif
ifneq ("$(OSS_TARBALL_PATH)", "")
	TARBALL_PATH_SECTION := -s $(OSS_TARBALL_PATH)
endif

# build .pkg
.PHONY: pkg
pkg:
	mkdir -p $(BUILDDIR)/
	cp ./build.assets/build-package.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	# arch and runtime are currently ignored on OS X
	# we pass them through for consistency - they will be dropped by the build script
	cd $(BUILDDIR) && ./build-package.sh -t oss -v $(VERSION) -p pkg -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)
	if [ -f e/Makefile ]; then $(MAKE) -C e pkg; fi

# build tsh client-only .pkg
.PHONY: pkg-tsh
pkg-tsh:
	mkdir -p $(BUILDDIR)/
	cp ./build.assets/build-package.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	# arch and runtime are currently ignored on OS X
	# we pass them through for consistency - they will be dropped by the build script
	cd $(BUILDDIR) && ./build-package.sh -t oss -v $(VERSION) -p pkg -a $(ARCH) -m tsh $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)

# build .rpm
.PHONY: rpm
rpm:
	mkdir -p $(BUILDDIR)/
	cp ./build.assets/build-package.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	cp -a ./build.assets/rpm-sign $(BUILDDIR)/
	cd $(BUILDDIR) && ./build-package.sh -t oss -v $(VERSION) -p rpm -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)
	if [ -f e/Makefile ]; then $(MAKE) -C e rpm; fi

# build unsigned .rpm (for testing)
.PHONY: rpm-unsigned
rpm-unsigned:
	$(MAKE) UNSIGNED_RPM=true rpm

# build .deb
.PHONY: deb
deb:
	mkdir -p $(BUILDDIR)/
	cp ./build.assets/build-package.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	cd $(BUILDDIR) && ./build-package.sh -t oss -v $(VERSION) -p deb -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)
	if [ -f e/Makefile ]; then $(MAKE) -C e deb; fi

# update Helm chart versions
# this isn't a 'proper' semver regex but should cover most cases
# the order of parameters in sed's extended regex mode matters; the
# dash (-) must be the last character for this to work as expected
.PHONY: update-helm-charts
update-helm-charts:
	sed -i -E "s/^  tag: [a-z0-9.-]+$$/  tag: $(VERSION)/" examples/chart/teleport/values.yaml
	sed -i -E "s/^  tag: [a-z0-9.-]+$$/  tag: $(VERSION)/" examples/chart/teleport-auto-trustedcluster/values.yaml
	sed -i -E "s/^  tag: [a-z0-9.-]+$$/  tag: $(VERSION)/" examples/chart/teleport-daemonset/values.yaml

.PHONY: ensure-webassets
ensure-webassets:
	@if [ ! -d $(shell pwd)/webassets/teleport/ ]; then \
		$(MAKE) init-webapps-submodules; \
	fi;

.PHONY: ensure-webassets-e
ensure-webassets-e:
	@if [ ! -d $(shell pwd)/webassets/e/teleport ]; then \
		$(MAKE) init-webapps-submodules-e; \
	fi;

.PHONY: init-webapps-submodules
init-webapps-submodules:
	echo "init webassets submodule"
	git submodule update --init webassets

.PHONY: init-webapps-submodules-e
init-webapps-submodules-e:
	echo "init webassets oss and enterprise submodules"
	git submodule update --init --recursive webassets

.PHONY: init-submodules-e
init-submodules-e: init-webapps-submodules-e
	git submodule init e
	git submodule update

.PHONY: update-vendor
update-vendor:
	$(GO_BINARY) mod tidy
	$(GO_BINARY) mod vendor
	# delete the vendored api package. In its place
	# create a symlink to the the original api package
	rm -r vendor/github.com/gravitational/teleport/api
	ln -s -r $(shell readlink -f api) vendor/github.com/gravitational/teleport

# update-webassets updates the minified code in the webassets repo using the latest webapps
# repo and creates a PR in the teleport repo to update webassets submodule.
.PHONY: update-webassets
update-webassets: WEBAPPS_BRANCH ?= 'master'
update-webassets: TELEPORT_BRANCH ?= 'master'
update-webassets:
	build.assets/webapps/update-teleport-webassets.sh -w $(WEBAPPS_BRANCH) -t $(TELEPORT_BRANCH)

# dronegen generates .drone.yml config
.PHONY: dronegen
dronegen:
	go run ./dronegen
