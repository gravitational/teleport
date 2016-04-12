# these are standard autotools variables, don't change them please
BUILDDIR ?= build
BINDIR ?= /usr/local/bin
DATADIR ?= /usr/local/share/teleport
ADDFLAGS ?=

GO15VENDOREXPERIMENT := 1
PKGPATH=github.com/gravitational/teleport
PWD ?= $(shell pwd)
ETCD_CERTS := $(realpath fixtures/certs)
ETCD_FLAGS := TELEPORT_TEST_ETCD_CONFIG='{"nodes": ["https://localhost:4001"], "key":"/teleport/test", "tls_key_file": "$(ETCD_CERTS)/proxy1-key.pem", "tls_cert_file": "$(ETCD_CERTS)/proxy1.pem", "tls_ca_file": "$(ETCD_CERTS)/ca.pem"}'
TELEPORT_DEBUG_TESTS ?= no
export

#
# Default target: builds all 3 executables and plaaces them in a current directory
#
.PHONY: all
all: build

.PHONY: build
build: flags teleport tctl tsh assets

.PHONY: tctl
tctl: 
	go build -o $(BUILDDIR)/tctl -i $(BUILDFLAGS) $(PKGPATH)/tool/tctl

.PHONY: teleport 
teleport: flags
	go build -o $(BUILDDIR)/teleport -i $(BUILDFLAGS) $(PKGPATH)/tool/teleport

.PHONY: tsh
tsh: 
	go build -o $(BUILDDIR)/tsh -i $(BUILDFLAGS) $(PKGPATH)/tool/tsh

.PHONY: install
install: build
	sudo cp -f $(BUILDDIR)/tctl      $(BINDIR)/
	sudo cp -f $(BUILDDIR)/tsh       $(BINDIR)/
	sudo cp -f $(BUILDDIR)/teleport  $(BINDIR)/
	sudo mkdir -p $(DATADIR)
	sudo cp -fr web/dist/* $(DATADIR)

.PHONY: goinstall
goinstall: flags
	go install $(BUILDFLAGS) $(PKGPATH)/tool/tctl
	go install $(BUILDFLAGS) $(PKGPATH)/tool/teleport
	go install $(PKGPATH)/tool/tsh


.PHONY: clean
clean:
	rm -rf $(BUILDDIR)

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
# tests everything: called by Jenkins
#
.PHONY: test
test: FLAGS ?= -cover
test: 
	go test -v $(PKGPATH)/integration
	go test -v $(PKGPATH)/tool/tsh/... \
			   $(PKGPATH)/lib/... \
			   $(PKGPATH)/tool/teleport... $(FLAGS) -tags test
	go vet ./tool/... ./lib/...


#
# source-release releases source distribution tarball for this particular version
#
.PHONY: source-release
source-release: LINKFLAGS := $(shell linkflags -verpkg=$(PKGPATH)/vendor/github.com/gravitational/version)
source-release: RELEASE := teleport-$(shell linkflags --tag)-src
source-release: RELEASEDIR := $(BUILDDIR)/$(RELEASE)
source-release: flags
	mkdir -p $(RELEASEDIR)/src/github.com/gravitational/teleport
	find -type f | grep -v node_modules | grep -v ./build | grep -v ./.git | grep -v .test$$ > $(BUILDDIR)/files.txt
	tar --transform "s_./_teleport/src/github.com/gravitational/teleport/_" -cvf $(BUILDDIR)/$(RELEASE).tar -T $(BUILDDIR)/files.txt
	sed 's_%BUILDFLAGS%_-ldflags "$(LINKFLAGS)"_' build.assets/release/Makefile > $(BUILDDIR)/Makefile
	tar --transform "s__teleport/_" -uvf $(BUILDDIR)/$(RELEASE).tar README.md LICENSE docs
	tar --transform "s_$(BUILDDIR)/_teleport/_" -uvf $(BUILDDIR)/$(RELEASE).tar $(BUILDDIR)/Makefile
	gzip $(BUILDDIR)/$(RELEASE).tar

#
# bianry-release releases binary distribution tarball for this particular version
#
.PHONY: binary-release
binary-release: LINKFLAGS := $(shell linkflags -verpkg=$(PKGPATH)/vendor/github.com/gravitational/version)
binary-release: RELEASE := teleport-$(shell linkflags --os-release)-bin
binary-release: RELEASEDIR := $(BUILDDIR)/$(RELEASE)
binary-release: build
	sed 's_%BUILDFLAGS%_-ldflags "$(LINKFLAGS)"_' build.assets/release/Makefile > $(BUILDDIR)/Makefile
	mkdir -p $(BUILDDIR)/$(RELEASE)/teleport/src/$(PKGPATH)/web $(BUILDDIR)/$(RELEASE)/teleport/build
	cp -r $(BUILDDIR)/Makefile LICENSE README.md docs $(BUILDDIR)/$(RELEASE)/teleport
	cp -r web/dist $(BUILDDIR)/$(RELEASE)/teleport/src/$(PKGPATH)/web
	cp -af $(BUILDDIR)/tctl $(BUILDDIR)/tsh $(BUILDDIR)/teleport $(BUILDDIR)/$(RELEASE)/teleport/build
	tar -czf $(BUILDDIR)/$(RELEASE).tar.gz -C $(BUILDDIR)/$(RELEASE) teleport

.PHONY: flags
flags:
	$(shell go install $(PKGPATH)/vendor/github.com/gravitational/version/cmd/linkflags)
	$(eval BUILDFLAGS := $(ADDFLAGS) -ldflags "-w $(shell linkflags -pkg=$(GOPATH)/src/$(PKGPATH) -verpkg=$(PKGPATH)/vendor/github.com/gravitational/version)")


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
