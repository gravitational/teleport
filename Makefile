TCD_NODE1 := http://127.0.0.1:4001
ETCD_NODES := ${ETCD_NODE1}
ETCD_FLAGS := TELEPORT_TEST_ETCD_NODES=${ETCD_NODES}
TARGETS=teleport tctl tsh

.PHONY: all install test test-with-etcd remove-temp files test-package update test-grep-package cover-package cover-package-with-etcd run profile sloccount set-etcd install-assets docs-serve

#
# This target can 
#
all: $(TARGETS)
teleport:
	go build -o teleport -a github.com/gravitational/teleport/tool/teleport
tctl:
	go build -o tctl -a github.com/gravitational/teleport/tool/tctl
tsh:
	go build -o tsh github.com/gravitational/teleport/tool/tsh

install: remove-temp-files
	go install github.com/gravitational/teleport/tool/teleport
	go install github.com/gravitational/teleport/tool/tctl
	go install github.com/gravitational/teleport/tool/tsh

clean:
	rm -f $(TARGETS)

#
# this target is used by Jenkins for production builds
#
.PHONY: production
production:
	$(MAKE) -C build.assets


test: install
	go test -v -test.parallel=0 $(shell go list ./... | grep -v /vendor/) -cover

test-with-etcd: install
	${ETCD_FLAGS} go test -v -test.parallel=0 $(shell go list ./... | grep -v /vendor/) -cover

remove-temp-files:
	find . -name flymake_* -delete

test-package: remove-temp-files install
	go test -v -test.parallel=0 ./$(p)

test-package-with-etcd: remove-temp-files install
	${ETCD_FLAGS} go test -v -test.parallel=0 ./$(p)

update:
	rm -rf Godeps/
	find . -iregex .*go | xargs sed -i 's:".*Godeps/_workspace/src/:":g'
	godep save -r ./...

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

# run-embedded starts a auth server, ssh node and proxy that allows web access 
# to all the nodes
run-embedded: install
	teleport --config=examples/embedded.yaml

# run-node starts a ssh node
run-node: install
	tctl token generate --output=/tmp/token --domain=localhost
	teleport --config=examples/node.yaml

# run-site-to-proxy starts a ssh node, auth server and reverse tunnel that connect outside of
# the organization server
run-site-to-proxy: install
	rm -f /tmp/teleport.auth.sock
	teleport --config=examples/embedded-proxy.yaml

# run proxy start s
run-proxy: install
	rm -f /tmp/teleport.proxy.auth.sock
	teleport --config=examples/proxy.yaml

trust-proxy:
#   get user and host SSH certificates from proxy's organization, note that we are connecting to proxy's auth server
#   that serves proxy's organization certs and not teleport's
	tctl --auth=unix:///tmp/teleport.proxy.auth.sock user-ca pub-key > /tmp/user.pubkey
	tctl --auth=unix:///tmp/teleport.proxy.auth.sock host-ca pub-key > /tmp/host.pubkey

#   add proxy's certs to teleport as trusted remote certificate authorities
	tctl remote-ca upsert --type=user --id=user.proxy.vendor.io --domain=proxy.vendor.io --path=/tmp/user.pubkey
	tctl remote-ca upsert --type=host --id=host.proxy.vendor.io --domain=proxy.vendor.io --path=/tmp/host.pubkey
	tctl remote-ca ls --type=user
	tctl remote-ca ls --type=host

#   now export teleport's host CA certificate and add it as a trusted cert for proxy
	tctl host-ca pub-key > /tmp/teleport.pubkey
	tctl --auth=unix:///tmp/teleport.proxy.auth.sock remote-ca upsert --type=host --id=host.auth.gravitational.io --domain=node1.gravitational.io --path=/tmp/teleport.pubkey
	tctl --auth=unix:///tmp/teleport.proxy.auth.sock remote-ca ls --type=host

profile:
	go tool pprof http://localhost:6060/debug/pprof/profile

sloccount:
	find . -path ./Godeps -prune -o -name "*.go" -print0 | xargs -0 wc -l

docs-serve:
	sleep 1 && sensible-browser http://127.0.0.1:32567 &
	mkdocs serve

docs-update:
	echo "# Auth Server Client\n\n" > docs/api.md
	echo "[Source file](https://github.com/gravitational/teleport/blob/master/auth/clt.go)" >> docs/api.md
	echo '```go' >> docs/api.md
	godoc github.com/gravitational/teleport/auth Client >> docs/api.md
	echo '```' >> docs/api.md

# Deploy teleport server to staging environment on AWS
.PHONY: deploy
deploy: all
	ansible-playbook -i deploy/hosts deploy/deploy.yaml

# Prepare a brand new AWS machine to host Teleport (run provision once, 
# then run deploy many times)
.PHONY: provision
provision:
	ansible-playbook -i deploy/hosts deploy/provision.yaml

.PHONY: jenkins
jenkins:
	curl -X POST -d TARGETENV=staging -d BRANCH=$$(git rev-parse --abbrev-ref HEAD) https://jenkins.gravitational.io/buildByToken/buildWithParameters?job=Teleport&token=ZeeYaeYuTh9quiu8eh3rieChohHoor8aib0oopoov0aewah8 
