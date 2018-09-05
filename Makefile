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
VERSION=3.0.0-alpha.9

# These are standard autotools variables, don't change them please
BUILDDIR ?= build
BINDIR ?= /usr/local/bin
DATADIR ?= /usr/local/share/teleport
ADDFLAGS ?=
PWD ?= `pwd`
GOCACHEDIR ?= `go env GOCACHE`
TELEPORT_DEBUG ?= no
GITTAG=v$(VERSION)
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s'

OS ?= `go env GOOS`
ARCH ?= `go env GOARCH`
RELEASE=teleport-$(GITTAG)-$(OS)-$(ARCH)-bin

# On Windows only build tsh. On all other platforms build teleport, tctl,
# and tsh.
BINARIES=$(BUILDDIR)/teleport $(BUILDDIR)/tctl $(BUILDDIR)/tsh
OS_MESSAGE = "Building Teleport binaries with GOOS=$(OS) GOARCH=$(ARCH)."
ifeq ("$(OS)","windows")
BINARIES=$(BUILDDIR)/tsh
endif

VERSRC = version.go gitref.go

# PAM support will only be built into Teleport if headers exist at build time.
PAM_MESSAGE = "Building Teleport binaries without PAM support."
ifneq ("$(wildcard /usr/include/security/pam_appl.h)","")
PAMFLAGS = -tags pam
PAM_MESSAGE = "Building Teleport binaries with PAM support."
endif

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
	@echo $(OS_MESSAGE)
	@echo $(PAM_MESSAGE)
	$(MAKE) $(BINARIES)

# By making these 3 targets below (tsh, tctl and teleport) PHONY we are solving
# several problems:
# * Build will rely on go build internal caching https://golang.org/doc/go1.10 at all times
# * Manual change detection was broken on a large dependency tree
# If you are considering changing this behavior, please consult with dev team first
.PHONY: $(BUILDDIR)/tctl
$(BUILDDIR)/tctl:
	go build $(PAMFLAGS) -o $(BUILDDIR)/tctl $(BUILDFLAGS) ./tool/tctl

.PHONY: $(BUILDDIR)/teleport
$(BUILDDIR)/teleport:
	go build $(PAMFLAGS) -o $(BUILDDIR)/teleport $(BUILDFLAGS) ./tool/teleport

.PHONY: $(BUILDDIR)/tsh
$(BUILDDIR)/tsh:
	GOOS=$(OS) go build $(PAMFLAGS) -o $(BUILDDIR)/tsh $(BUILDFLAGS) ./tool/tsh

#
# make full - Builds Teleport binaries with the built-in web assets and
# places them into $(BUILDDIR). On Windows, this target is skipped because
# only tsh is built.
#
.PHONY:full
full: all $(BUILDDIR)/webassets.zip
ifneq ("$(OS)", "windows")
	cat $(BUILDDIR)/webassets.zip >> $(BUILDDIR)/teleport
	rm -fr $(BUILDDIR)/webassets.zip
	zip -q -A $(BUILDDIR)/teleport
	if [ -f e/Makefile ]; then $(MAKE) -C e full; fi
endif

#
# make clean - Removed all build artifacts.
#
.PHONY: clean
clean:
	rm -rf $(BUILDDIR)
	rm -rf $(GOCACHEDIR)
	rm -rf teleport
	rm -rf *.gz
	rm -rf *.zip
	rm -f gitref.go
	rm -rf `go env GOPATH`/pkg/`go env GOHOSTOS`_`go env GOARCH`/github.com/gravitational/teleport*
	@if [ -f e/Makefile ]; then $(MAKE) -C e clean; fi

#
# make release - Produces a binary release tarball.
#
.PHONY:
export
release:
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
	@echo "\nCREATED: $(RELEASE).tar.gz"
	@if [ -f e/Makefile ]; then $(MAKE) -C e release; fi

#
# make release-windows - Produces a binary release tarball containing teleport,
# tctl, and tsh.
#
.PHONY:
release-windows: clean all
	mkdir teleport
	cp -rf $(BUILDDIR)/* \
		README.md \
		CHANGELOG.md \
		teleport/
	mv teleport/tsh teleport/tsh.exe
	echo $(GITTAG) > teleport/VERSION
	zip -9 -y -r -q $(RELEASE).zip teleport/
	rm -rf teleport/
	@echo "\nCREATED: $(RELEASE).zip"

#
# Builds docs using containerized mkdocs
#
.PHONY:docs
docs:
	$(MAKE) -C build.assets docs

#
# Runs the documentation site inside a container on localhost with live updates
# Convenient for editing documentation.
#
.PHONY:run-docs
run-docs:
	$(MAKE) -C build.assets run-docs

#
# tests everything: called by Jenkins
#
.PHONY: test
test: FLAGS ?=
test: $(VERSRC)
	go test -v ./tool/tsh/... \
			   ./lib/... \
			   ./tool/teleport... $(FLAGS) $(ADDFLAGS)
	go vet ./tool/... ./lib/...

#
# integration tests. need a TTY to work and not compatible with a race detector
#
.PHONY: integration
integration:
	@echo KUBECONFIG is: $(KUBECONFIG), TEST_KUBE: $(TEST_KUBE)
	go test $(PAMFLAGS) -v ./integration/... -check.v

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
	cd web/dist ; zip -qr ../../$(BUILDDIR)/webassets.zip .
endif

.PHONY: test-package
test-package: remove-temp-files
	go test -v -test.parallel=0 ./$(p)

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

# Dockerized build: usefule for making Linux releases on OSX
.PHONY:docker
docker:
	make -C build.assets

# Interactively enters a Docker container (which you can build and run Teleport inside of)
.PHONY:enter
enter:
	make -C build.assets enter

PROTOC_VER ?= 3.0.0
GOGO_PROTO_TAG ?= v0.3
PLATFORM := linux-x86_64
GRPC_API := lib/events
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
	docker run -v $(shell pwd):/go/src/github.com/gravitational/teleport $(BUILDBOX_TAG) make -C /go/src/github.com/gravitational/teleport buildbox-grpc

# proto generates GRPC stuff inside buildbox
.PHONY: buildbox-grpc
buildbox-grpc:
# standard GRPC output
	echo $$PROTO_INCLUDE
	cd $(GRPC_API) && protoc -I=.:$$PROTO_INCLUDE \
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


.PHONY: image
image:
	cp ./build.assets/charts/Dockerfile $(BUILDDIR)/
	cd $(BUILDDIR) && docker build . -t quay.io/gravitational/teleport:$(VERSION)
	if [ -f e/Makefile ]; then $(MAKE) -C e image; fi

.PHONY: publish
publish:
	docker push quay.io/gravitational/teleport:$(VERSION)
	if [ -f e/Makefile ]; then $(MAKE) -C e publish; fi

.PHONY: print-version
print-version:
	@echo $(VERSION)

.PHONY: chart-ent
chart-ent:
	$(MAKE) -C e chart
