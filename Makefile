# Update this variable, then run 'make setver'
# Naming convention:
#	for stable releases we use "1.0.0" format
#   for pre-releases, we use   "1.0.0-beta.2" format
VERSION=1.1.0

# These are standard autotools variables, don't change them please
BUILDDIR ?= build
BINDIR ?= /usr/local/bin
DATADIR ?= /usr/local/share/teleport
ADDFLAGS ?=

PWD ?= $(shell pwd)
ETCD_CERTS := $(realpath fixtures/certs)
ETCD_FLAGS := TELEPORT_TEST_ETCD_CONFIG='{"nodes": ["https://localhost:4001"], "key":"/teleport/test", "tls_key_file": "$(ETCD_CERTS)/proxy1-key.pem", "tls_cert_file": "$(ETCD_CERTS)/proxy1.pem", "tls_ca_file": "$(ETCD_CERTS)/ca.pem"}'
TELEPORT_DEBUG ?= no
GITTAG=v$(VERSION)
RELEASE := teleport-$(GITTAG)-$(shell go env GOOS)-$(shell go env GOARCH)-bin

export

$(eval BUILDFLAGS := $(ADDFLAGS) -ldflags -w)

#
# Default target: builds all 3 executables and plaaces them in a current directory
#
.PHONY: all
all: setver teleport tctl tsh assets
	cp -f build.assets/release.mk $(BUILDDIR)/Makefile

.PHONY: tctl
tctl: 
	go build -o $(BUILDDIR)/tctl -i $(BUILDFLAGS) ./tool/tctl

.PHONY: teleport 
teleport:
	go build -o $(BUILDDIR)/teleport -i $(BUILDFLAGS) ./tool/teleport

.PHONY: tsh
tsh: 
	go build -o $(BUILDDIR)/tsh -i $(BUILDFLAGS) ./tool/tsh

#
# make install will installs system-wide teleport 
# 
.PHONY: install
install: build
	@echo "\n** Make sure to run 'make install' as root! **\n"
	cp -f $(BUILDDIR)/tctl      $(BINDIR)/
	cp -f $(BUILDDIR)/tsh       $(BINDIR)/
	cp -f $(BUILDDIR)/teleport  $(BINDIR)/
	mkdir -p $(DATADIR)
	cp -fr web/dist/* $(DATADIR)

.PHONY: goinstall
goinstall:
	go install $(BUILDFLAGS) ./tool/tctl
	go install $(BUILDFLAGS) ./tool/teleport
	go install $(BUILDFLAGS) ./tool/tsh


.PHONY: clean
clean:
	rm -rf $(BUILDDIR)
	rm -rf teleport

.PHONY: assets
assets:
	rm -rf $(BUILDDIR)/app
	rm -f web/dist/app/app
	cp -r web/dist/app $(BUILDDIR)
	cp web/dist/index.html $(BUILDDIR)
	cp README.md $(BUILDDIR)

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
test: FLAGS ?= -cover
test: 
	go test -v ./tool/tsh/... \
			   ./lib/... \
			   ./tool/teleport... $(FLAGS) -tags test
	go vet ./tool/... ./lib/...

#
# integration tests. need a TTY to work and not compatible with a race detector
#
.PHONY: integration
integration: 
	go test -v ./integration/...


# make setver - bump the version of teleport
#	Reads the version from version.mk, updates version.go and
#	assigns a git tag to the currently checked out tree
.PHONY: setver
setver:
	$(MAKE) -f version.mk setver

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

#
# make release - produces a binary release tarball 
#	
.PHONY: release
release: clean setver all
	cp -rf $(BUILDDIR) teleport
	@echo $(GITTAG) > teleport/VERSION
	tar -czf $(RELEASE).tar.gz teleport
	rm -rf teleport
	@echo "\n"
	@echo "CREATED: $(RELEASE).tar.gz"


.PHONY: test-with-etcd
test-with-etcd: remove-temp-files
	${ETCD_FLAGS} go test -v -test.parallel=0 $(shell go list ./... | grep -v /vendor/) -cover

.PHONY: test-package
test-package: remove-temp-files
	go test -v -test.parallel=0 ./$(p)

.PHONY: tets-package-with-etcd
test-package-with-etcd: remove-temp-files
	${ETCD_FLAGS} go test -v -test.parallel=0 ./$(p)

.PHONY: test-grep-package-with-etcd
test-grep-package-with-etcd: remove-temp-files
	${ETCD_FLAGS} go test -v -test.parallel=0 ./$(p) -check.f=$(e)

.PHONY: test-grep-package
test-grep-package: remove-temp-files
	go test -v ./$(p) -check.f=$(e)

.PHONY: cover-package
cover-package: remove-temp-files
	go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

.PHONY: cover-package-with-etcd
cover-package-with-etcd: remove-temp-files
	${ETCD_FLAGS} go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

.PHONY: profile
profile:
	go tool pprof http://localhost:6060/debug/pprof/profile

.PHONY: sloccount
sloccount:
	find . -path ./vendor -prune -o -name "*.go" -print0 | xargs -0 wc -l

# start-test-etcd starts test etcd node using tls certificates
.PHONY: start-test-etcd
start-test-etcd:
	docker run -d -p 4001:4001 -p 2380:2380 -p 2379:2379 -v $(ETCD_CERTS):/certs quay.io/coreos/etcd:v2.2.5  -name etcd0 -advertise-client-urls https://localhost:2379,https://localhost:4001  -listen-client-urls https://0.0.0.0:2379,https://0.0.0.0:4001  -initial-advertise-peer-urls https://localhost:2380  -listen-peer-urls https://0.0.0.0:2380  -initial-cluster-token etcd-cluster-1  -initial-cluster etcd0=https://localhost:2380  -initial-cluster-state new --cert-file=/certs/etcd1.pem --key-file=/certs/etcd1-key.pem --peer-cert-file=/certs/etcd1.pem --peer-key-file=/certs/etcd1-key.pem --peer-client-cert-auth --peer-trusted-ca-file=/certs/ca.pem -client-cert-auth

.PHONY: remove-temp-files
remove-temp-files:
	find . -name flymake_* -delete
