BUILDDIR ?= out
GO15VENDOREXPERIMENT := 1
PKGPATH=github.com/gravitational/teleport
PWD ?= $(shell pwd)
ETCD_CERTS := $(realpath fixtures/certs)
ETCD_FLAGS := TELEPORT_TEST_ETCD_CONFIG='{"nodes": ["https://localhost:4001"], "key":"/teleport/test", "tls_key_file": "$(ETCD_CERTS)/proxy1-key.pem", "tls_cert_file": "$(ETCD_CERTS)/proxy1.pem", "tls_ca_file": "$(ETCD_CERTS)/ca.pem"}'
TELEPORT_DEBUG_TESTS ?= no
OUT := out
GO15VENDOREXPERIMENT := 1
INSTALL_BIN_DIR=/usr/bin
INSTALL_ASSETS_DIR=/usr/share/teleport
export

.PHONY: install test test-with-etcd remove-temp files test-package update test-grep-package cover-package cover-package-with-etcd run profile sloccount set-etcd 

#
# Default target: builds all 3 executables and plaaces them in a current directory
#
.PHONY: all
all: flags teleport tctl tsh assets

.PHONY: tctl
tctl: 
	go build -o $(OUT)/tctl -i $(BUILDFLAGS) $(PKGPATH)/tool/tctl

.PHONY: teleport 
teleport: flags
	go build -o $(OUT)/teleport -i $(BUILDFLAGS) $(PKGPATH)/tool/teleport

.PHONY: tsh
tsh: 
	go build -o $(OUT)/tsh -i $(BUILDFLAGS) $(PKGPATH)/tool/tsh

install: 
	$(eval BUILDFLAGS=-ldflags -w)
	go build -o $(OUT)/tctl     $(BUILDFLAGS) $(PKGPATH)/tool/tctl
	go build -o $(OUT)/teleport $(BUILDFLAGS) $(PKGPATH)/tool/teleport
	go build -o $(OUT)/tsh      $(BUILDFLAGS) $(PKGPATH)/tool/tsh
	sudo cp -f $(OUT)/tctl      $(INSTALL_BIN_DIR)/
	sudo cp -f $(OUT)/tsh       $(INSTALL_BIN_DIR)/
	sudo cp -f $(OUT)/teleport  $(INSTALL_BIN_DIR)/
	sudo mkdir -p $(INSTALL_ASSETS_DIR)
	sudo cp -fr web/dist/* $(INSTALL_ASSETS_DIR)
	go build -ldflags $(TELEPORT_LINKFLAGS) -o $(BUILDDIR)/tctl $(BUILDFLAGS) -i $(PKGPATH)/tool/tctl


.PHONY: clean
clean:
	rm -rf $(BUILDDIR)

.PHONY: assets
assets:
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
test: FLAGS ?= -cover
test: 
	go test -v $(PKGPATH)/tool/tsh/... \
			   $(PKGPATH)/lib/... \
			   $(PKGPATH)/tool/teleport... $(FLAGS)
	go vet ./tool/... ./lib/...

flags:
	$(shell go install $(PKGPATH)/vendor/github.com/gravitational/version/cmd/linkflags)
	$(eval TELEPORT_LINKFLAGS := "$(shell linkflags -pkg=$(GOPATH)/src/$(PKGPATH) -verpkg=$(PKGPATH)/vendor/github.com/gravitational/version)")


test-with-etcd: install
	${ETCD_FLAGS} go test -v -test.parallel=0 $(shell go list ./... | grep -v /vendor/) -cover

test-package: remove-temp-files install
	go test -v -test.parallel=0 ./$(p)

test-package-with-etcd: remove-temp-files install
	${ETCD_FLAGS} go test -v -test.parallel=0 ./$(p)

test-grep-package-with-etcd: remove-temp-files install
	${ETCD_FLAGS} go test -v -test.parallel=0 ./$(p) -check.f=$(e)


test-grep-package: remove-temp-files install
	go test -v ./$(p) -check.f=$(e)

cover-package: remove-temp-files
	go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

cover-package-with-etcd: remove-temp-files
	${ETCD_FLAGS} go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

profile:
	go tool pprof http://localhost:6060/debug/pprof/profile

sloccount:
	find . -path ./vendor -prune -o -name "*.go" -print0 | xargs -0 wc -l

# start-test-etcd starts test etcd node using tls certificates
start-test-etcd:
	docker run -d -p 4001:4001 -p 2380:2380 -p 2379:2379 -v $(ETCD_CERTS):/certs quay.io/coreos/etcd:v2.2.5  -name etcd0 -advertise-client-urls https://localhost:2379,https://localhost:4001  -listen-client-urls https://0.0.0.0:2379,https://0.0.0.0:4001  -initial-advertise-peer-urls https://localhost:2380  -listen-peer-urls https://0.0.0.0:2380  -initial-cluster-token etcd-cluster-1  -initial-cluster etcd0=https://localhost:2380  -initial-cluster-state new --cert-file=/certs/etcd1.pem --key-file=/certs/etcd1-key.pem --peer-cert-file=/certs/etcd1.pem --peer-key-file=/certs/etcd1-key.pem --peer-client-cert-auth --peer-trusted-ca-file=/certs/ca.pem -client-cert-auth

