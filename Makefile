ETCD_NODE1 := http://127.0.0.1:4001
ETCD_NODES := ${ETCD_NODE1}
ETCD_FLAGS := TELEPORT_TEST_ETCD_NODES=${ETCD_NODES}
OUT=out
export GO15VENDOREXPERIMENT=1

.PHONY: install test test-with-etcd remove-temp files test-package update test-grep-package cover-package cover-package-with-etcd run profile sloccount set-etcd install-assets docs-serve

#
# Default target: builds all 3 executables and plaaces them in a current directory
#
.PHONY: all
all: teleport tctl tsh

.PHONY: tctl
tctl: 
	go build -o $(OUT)/tctl -i github.com/gravitational/teleport/tool/tctl

.PHONY: teleport
teleport: 
	ln -f -s $$(pwd)/web/dist/app /var/lib/teleport/app
	ln -f -s $$(pwd)/web/dist/index.html /var/lib/teleport/index.html
	go build -o $(OUT)/teleport -i github.com/gravitational/teleport/tool/teleport

.PHONY: tsh
tsh: 
	go build -o $(OUT)/tsh -i github.com/gravitational/teleport/tool/tsh

install: remove-temp-files
	go install github.com/gravitational/teleport/tool/teleport
	go install github.com/gravitational/teleport/tool/tctl
	go install github.com/gravitational/teleport/tool/t

clean:
	rm -rf $(OUT)

#
# this target is used by Jenkins for production builds
#
.PHONY: production
production: clean
	$(MAKE) -C build.assets

#
# tests everything: called by Jenkins
#
test: 
	go test -v github.com/gravitational/teleport/tool/t/...
	go test -v github.com/gravitational/teleport/lib/... -cover
	go test -v github.com/gravitational/teleport/tool/teleport... -cover


test-with-etcd: install
	${ETCD_FLAGS} go test -v -test.parallel=0 $(shell go list ./... | grep -v /vendor/) -cover

remove-temp-files:
	find . -name flymake_* -delete

test-package: remove-temp-files install
	go test -v -test.parallel=0 ./$(p)

test-package-with-etcd: remove-temp-files install
	${ETCD_FLAGS} go test -v -test.parallel=0 ./$(p)


test-grep-package: remove-temp-files install
	go test -v ./$(p) -check.f=$(e)

cover-package: remove-temp-files
	go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

cover-package-with-etcd: remove-temp-files
	${ETCD_FLAGS} go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

pack-teleport: DIR := $(shell mktemp -d)
pack-teleport: pkg teleport
	cp assets/build/orbit.manifest.json $(DIR)
	mkdir -p $(DIR)/rootfs/usr/bin
	mkdir -p $(DIR)/rootfs/usr/bin $(DIR)/rootfs/etc/web-assets/
	cp -r ./assets/web/* $(DIR)/rootfs/etc/web-assets/
	cp $(GOPATH)/bin/teleport $(DIR)/rootfs/usr/bin
	cp $(GOPATH)/bin/tctl $(DIR)/rootfs/usr/bin
	gravity package import $(DIR) $(PKG) --check-manifest
	rm -rf $(DIR)

pkg:
	@if [ "$$PKG" = "" ] ; then echo "ERROR: enter PKG parameter:\n\nmake publish PKG=<name>:<sem-ver>, e.g. teleport:0.0.1\n\n" && exit 255; fi


profile:
	go tool pprof http://localhost:6060/debug/pprof/profile

sloccount:
	find . -path ./vendor -prune -o -name "*.go" -print0 | xargs -0 wc -l

#
# Deploy teleport server to staging environment on AWS
# WARNING: this step is called by CI/CD. You must execute make production first
.PHONY: deploy
deploy:
	ansible-playbook -i deploy/hosts deploy/deploy.yaml

# Prepare a brand new AWS machine to host Teleport (run provision once, 
# then run deploy many times)
.PHONY: provision
provision:
	ansible-playbook -i deploy/hosts deploy/provision.yaml

.PHONY: jenkins
jenkins:
	curl -X POST -d TARGETENV=staging -d BRANCH=$$(git rev-parse --abbrev-ref HEAD) https://jenkins.gravitational.io/buildByToken/buildWithParameters?job=Teleport&token=ZeeYaeYuTh9quiu8eh3rieChohHoor8aib0oopoov0aewah8 
