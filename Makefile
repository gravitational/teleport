# Make targets:
#
#  all    : builds all binaries in development mode, without web assets (default)
#  full   : builds all binaries for PRODUCTION use
#  release: prepares a release tarball
#  clean  : removes all buld artifacts
#  test   : runs tests

# To update the Teleport version, update VERSION variable:
# Naming convention:
#	for stable releases we use "1.0.0" format
#   for pre-releases, we use   "1.0.0-beta.2" format
VERSION=4.4.0-dev

DOCKER_IMAGE ?= quay.io/gravitational/teleport

# These are standard autotools variables, don't change them please
BUILDDIR ?= build
BINDIR ?= /usr/local/bin
DATADIR ?= /usr/local/share/teleport
ADDFLAGS ?=
PWD ?= `pwd`
GOPKGDIR ?= `go env GOPATH`/pkg/`go env GOHOSTOS`_`go env GOARCH`/github.com/gravitational/teleport*
TELEPORT_DEBUG ?= no
GITTAG=v$(VERSION)
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s'
CGOFLAG ?= CGO_ENABLED=1
GO_LINTERS ?= "unused,govet,typecheck,deadcode,goimports,varcheck,structcheck,bodyclose,staticcheck,ineffassign,unconvert,misspell,gosimple"

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
FIPS ?=
RELEASE=teleport-$(GITTAG)-$(OS)-$(ARCH)-bin

# FIPS support must be requested at build time.
FIPS_MESSAGE := "without FIPS support"
ifneq ("$(FIPS)","")
FIPS_TAG := fips
FIPS_MESSAGE := "with FIPS support"
endif

# PAM support will only be built into Teleport if headers exist at build time.
PAM_MESSAGE := "without PAM support"
ifneq ("$(wildcard /usr/include/security/pam_appl.h)","")
PAM_TAG := pam
PAM_MESSAGE := "with PAM support"
endif

# BPF support will only be built into Teleport if headers exist at build time.
BPF_MESSAGE := "without BPF support"
ifneq ("$(wildcard /usr/include/bcc/libbpf.h)","")
BPF_TAG := bpf
BPF_MESSAGE := "with BPF support"
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
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" -o $(BUILDDIR)/tctl $(BUILDFLAGS) ./tool/tctl

.PHONY: $(BUILDDIR)/teleport
$(BUILDDIR)/teleport: ensure-webassets
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" -o $(BUILDDIR)/teleport $(BUILDFLAGS) ./tool/teleport

.PHONY: $(BUILDDIR)/tsh
$(BUILDDIR)/tsh:
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -tags "$(PAM_TAG) $(FIPS_TAG)" -o $(BUILDDIR)/tsh $(BUILDFLAGS) ./tool/tsh

#
# make full - Builds Teleport binaries with the built-in web assets and
# places them into $(BUILDDIR). On Windows, this target is skipped because
# only tsh is built.
#
.PHONY:full
full: all $(BUILDDIR)/webassets.zip
ifneq ("$(OS)", "windows")
	@echo "---> Attaching OSS web assets."
	cat $(BUILDDIR)/webassets.zip >> $(BUILDDIR)/teleport
	rm -fr $(BUILDDIR)/webassets.zip
	zip -q -A $(BUILDDIR)/teleport
endif

#
# make clean - Removed all build artifacts.
#
.PHONY: clean
clean:
	@echo "---> Cleaning up OSS build artifacts."
	rm -rf $(BUILDDIR)
	-go clean -cache
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
	@if [ -f e/Makefile ]; then $(MAKE) -C e release; fi

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
# Builds docs using containerized mkdocs
#
.PHONY:docs
docs: docs-test
	$(MAKE) -C build.assets docs

#
# Runs the documentation site inside a container on localhost with live updates
# Convenient for editing documentation.
#
.PHONY:run-docs
run-docs:
	$(MAKE) -C build.assets run-docs

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
docs-test: docs-test-whitespace docs-test-links

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
# Run milv in docs to detect broken links.
# milv is installed if missing.
#
.PHONY:docs-test-links
docs-test-links: DOCS_FOLDERS := $(shell find . -name milv.config.yaml -exec dirname {} \;)
docs-test-links:
	go get -v github.com/magicmatatjahu/milv
	for docs_dir in $(DOCS_FOLDERS); do \
		echo "running milv in $${docs_dir}"; \
		cd $${docs_dir} && milv ; cd $(PWD); \
	done

#
# tests everything: called by Jenkins
#
.PHONY: test
test: ensure-webassets
test: FLAGS ?= '-race'
test: PACKAGES := $(shell go list ./... | grep -v integration)
test: $(VERSRC)
	go test -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" $(PACKAGES) $(FLAGS) $(ADDFLAGS)

#
# Integration tests. Need a TTY to work.
#
.PHONY: integration
integration: FLAGS ?= -v -race
integration:
	@echo KUBECONFIG is: $(KUBECONFIG), TEST_KUBE: $(TEST_KUBE)
	go test -tags "$(PAM_TAG) $(FIPS_TAG) $(BPF_TAG)" ./integration/... $(FLAGS)

#
# Lint the Go code.
# By default lint scans the entire repo. Pass FLAGS='--new' to only scan local
# changes (or last commit).
#
.PHONY: lint
lint: FLAGS ?=
lint:
	golangci-lint run \
		--disable-all \
		--exclude-use-default \
		--exclude='S1002: should omit comparison to bool constant' \
		--skip-dirs vendor \
		--uniq-by-line=false \
		--max-same-issues=0 \
		--max-issues-per-linter 0 \
		--timeout=5m \
		--enable $(GO_LINTERS) \
		$(FLAGS)

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
$(BUILDDIR)/webassets.zip:
ifneq ("$(OS)", "windows")
	@echo "---> Building OSS web assets."
	cd webassets/teleport/ ; zip -qr ../../$(BUILDDIR)/webassets.zip .
endif

.PHONY: test-package
test-package: remove-temp-files
	go test -v ./$(p)

.PHONY: test-grep-package
test-grep-package: remove-temp-files
	go test -v ./$(p) -check.f=$(e)

.PHONY: cover-package
cover-package: remove-temp-files
	go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

.PHONY: profile
profile:
	go tool pprof http://localhost:6060/debug/pprof/profile

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
docker-binaries:
	make -C build.assets build-binaries

# Interactively enters a Docker container (which you can build and run Teleport inside of)
.PHONY:enter
enter:
	make -C build.assets enter

PROTOC_VER ?= 3.6.1
GOGO_PROTO_TAG ?= v1.1.1
PLATFORM := linux-x86_64
BUILDBOX_TAG := teleport-grpc-buildbox:0.0.1

# buildbox builds docker buildbox image used to compile binaries and generate GRPc stuff
.PHONY: buildbox
buildbox:
	cd build.assets/grpc && docker build \
          --build-arg PROTOC_VER=$(PROTOC_VER) \
          --build-arg GOGO_PROTO_TAG=$(GOGO_PROTO_TAG) \
          --build-arg PLATFORM=$(PLATFORM) \
          -t $(BUILDBOX_TAG) .

# proto generates GRPC defs from service definitions
.PHONY: grpc
grpc: buildbox
	docker run \
		--rm \
		-v $(shell pwd):/go/src/github.com/gravitational/teleport $(BUILDBOX_TAG) \
		make -C /go/src/github.com/gravitational/teleport buildbox-grpc

# proto generates GRPC stuff inside buildbox
.PHONY: buildbox-grpc
buildbox-grpc:
# standard GRPC output
	echo $$PROTO_INCLUDE
	find lib/ -iname *.proto | xargs clang-format -i -style='{ColumnLimit: 100, IndentWidth: 4, Language: Proto}'

	cd lib/events && protoc -I=.:$$PROTO_INCLUDE \
	  --gofast_out=plugins=grpc:.\
    *.proto

	cd lib/services && protoc -I=.:$$PROTO_INCLUDE \
	  --gofast_out=plugins=grpc:.\
    *.proto

	cd lib/auth/proto && protoc -I=.:$$PROTO_INCLUDE \
	  --gofast_out=plugins=grpc:.\
    *.proto

	cd lib/wrappers && protoc -I=.:$$PROTO_INCLUDE \
	  --gofast_out=plugins=grpc:.\
    *.proto

.PHONY: goinstall
goinstall:
	go install $(BUILDFLAGS) \
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
image: docker-binaries
	cp ./build.assets/charts/Dockerfile $(BUILDDIR)/
	cd $(BUILDDIR) && docker build --no-cache . -t $(DOCKER_IMAGE):$(VERSION)
	if [ -f e/Makefile ]; then $(MAKE) -C e image; fi

.PHONY: publish
publish: image
	docker push $(DOCKER_IMAGE):$(VERSION)
	if [ -f e/Makefile ]; then $(MAKE) -C e publish; fi

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
	cp ./build.assets/build-package.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	# arch and runtime are currently ignored on OS X
	# we pass them through for consistency - they will be dropped by the build script
	cd $(BUILDDIR) && ./build-package.sh -t oss -v $(VERSION) -p pkg -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)
	if [ -f e/Makefile ]; then $(MAKE) -C e pkg; fi

# build tsh client-only .pkg
.PHONY: pkg-tsh
pkg-tsh:
	cp ./build.assets/build-package.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	# arch and runtime are currently ignored on OS X
	# we pass them through for consistency - they will be dropped by the build script
	cd $(BUILDDIR) && ./build-package.sh -t oss -v $(VERSION) -p pkg -a $(ARCH) -m tsh $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)

# build .rpm
.PHONY: rpm
rpm:
	cp ./build.assets/build-package.sh $(BUILDDIR)/
	chmod +x $(BUILDDIR)/build-package.sh
	cd $(BUILDDIR) && ./build-package.sh -t oss -v $(VERSION) -p rpm -a $(ARCH) $(RUNTIME_SECTION) $(TARBALL_PATH_SECTION)
	if [ -f e/Makefile ]; then $(MAKE) -C e rpm; fi

# build .deb
.PHONY: deb
deb:
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
	sed -i -E "s/^teleportVersion: [a-z0-9.-]+$$/teleportVersion: $(VERSION)/" examples/chart/teleport-demo/values.yaml

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
