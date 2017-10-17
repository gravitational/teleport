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
VERSION=2.3.5

# These are standard autotools variables, don't change them please
BUILDDIR ?= build
BINDIR ?= /usr/local/bin
DATADIR ?= /usr/local/share/teleport
ADDFLAGS ?=
PWD ?= `pwd`
TELEPORT_DEBUG ?= no
GITTAG=v$(VERSION)
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s'

ARCH=`go env GOOS`-`go env GOARCH`
RELEASE=teleport-$(GITTAG)-$(ARCH)-bin
BINARIES=$(BUILDDIR)/tsh $(BUILDDIR)/teleport $(BUILDDIR)/tctl

VERSRC = version.go gitref.go
LIBS = $(shell find lib -type f -name '*.go') *.go
TCTLSRC = $(shell find tool/tctl -type f -name '*.go')
TELEPORTSRC = $(shell find tool/teleport -type f -name '*.go')
TSHSRC = $(shell find tool/tsh -type f -name '*.go')

#
# 'make all' builds all 3 executables and plaaces them in a current directory
#
# IMPORTANT: the binaries will not contain the web UI assets and `teleport`
#            won't start without setting the environment variable DEBUG=1
#            This is the default build target for convenience of working on
#            a web UI.
.PHONY: all
all: $(VERSRC)
	go install $(BUILDFLAGS) ./lib/...
	$(MAKE) -s -j 4 $(BINARIES)

$(BUILDDIR)/tctl: $(LIBS) $(TCTLSRC)
	go build -o $(BUILDDIR)/tctl -i $(BUILDFLAGS) ./tool/tctl

$(BUILDDIR)/teleport: $(LIBS) $(TELEPORTSRC)
	go build -o $(BUILDDIR)/teleport -i $(BUILDFLAGS) ./tool/teleport

$(BUILDDIR)/tsh: $(LIBS) $(TSHSRC)
	go build -o $(BUILDDIR)/tsh -i $(BUILDFLAGS) ./tool/tsh

#
# make full - builds the binary with the built-in web assets and places it
#     into $(BUILDDIR)
#
.PHONY:full
full: all $(BUILDDIR)/webassets.zip
	cat $(BUILDDIR)/webassets.zip >> $(BUILDDIR)/teleport
	rm -fr $(BUILDDIR)/webassets.zip
	zip -q -A $(BUILDDIR)/teleport


.PHONY: clean
clean:
	rm -rf $(BUILDDIR)
	rm -rf teleport
	rm -rf *.gz
	rm -f gitref.go
	rm -rf `go env GOPATH`/pkg/`go env GOHOSTOS`_`go env GOARCH`/github.com/gravitational/teleport*
	@if [ -f e/Makefile ]; then $(MAKE) -C e clean; fi

#
# make release - produces a binary release tarball
#
.PHONY:
export
release: clean full
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
	if [ -f e/Makefile ]; then $(MAKE) -C e release; fi

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
	go test -v ./integration/...

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
	cd web/dist ; zip -qr ../../$(BUILDDIR)/webassets.zip .

.PHONY: test-package
test-package: remove-temp-files
	go test -v -test.parallel=0 ./$(p)

.PHONY: test-grep-package
test-grep-package: remove-temp-files
	go test -v ./$(p) -check.f=$(e)

.PHONY: test-dynamo
test-dynamo:
	go test -v ./lib/... -tags dynamodb

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
GRPC_GATEWAY_TAG ?= v1.1.0
PLATFORM := linux-x86_64
GRPC_API := lib/events
BUILDBOX_TAG := teleport-grpc-buildbox:0.0.1

# buildbox builds docker buildbox image used to compile binaries and generate GRPc stuff
.PHONY: buildbox
buildbox:
	cd build.assets/grpc && docker build \
          --build-arg PROTOC_VER=$(PROTOC_VER) \
          --build-arg GOGO_PROTO_TAG=$(GOGO_PROTO_TAG) \
          --build-arg GRPC_GATEWAY_TAG=$(GRPC_GATEWAY_TAG) \
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
