TCD_NODE1 := http://127.0.0.1:4001
ETCD_NODES := ${ETCD_NODE1}
ETCD_FLAGS := TELEPORT_TEST_ETCD_NODES=${ETCD_NODES}

.PHONY: install test test-with-etcd remove-temp files test-package update test-grep-package cover-package cover-package-with-etcd run profile sloccount set-etcd install-assets docs-serve

install: teleport telescope

teleport: remove-temp-files
	go install github.com/gravitational/teleport/tool/teleport
	go install github.com/gravitational/teleport/tool/tctl

telescope: remove-temp-files
	go install github.com/gravitational/teleport/tool/telescope/telescope
	go install github.com/gravitational/teleport/tool/tscopectl

test: install
	go test -v -test.parallel=0 ./... -cover

test-with-etcd: install
	${ETCD_FLAGS} go test -v -test.parallel=0 ./... -cover

remove-temp-files:
	find . -name flymake_* -delete

test-package: remove-temp-files
	go test -v -test.parallel=0 ./$(p)

test-package-with-etcd: remove-temp-files
	${ETCD_FLAGS} go test -v -test.parallel=0 ./$(p)

update:
	rm -rf Godeps/
	find . -iregex .*go | xargs sed -i 's:".*Godeps/_workspace/src/:":g'
	godep save -r ./...

test-grep-package: remove-temp-files
	go test -v ./$(p) -check.f=$(e)

cover-package: remove-temp-files
	go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

cover-package-with-etcd: remove-temp-files
	${ETCD_FLAGS} go test -v ./$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

pack-teleport: DIR := $(shell mktemp -d)
pack-teleport: pkg teleport
	cp assets/build/teleport/orbit.manifest.json $(DIR)
	mkdir -p $(DIR)/rootfs/usr/bin
	cp $(GOPATH)/bin/teleport $(DIR)/rootfs/usr/bin
	cp $(GOPATH)/bin/tctl $(DIR)/rootfs/usr/bin
	orbit pack $(DIR) $(PKG)
	rm -rf $(DIR)

pack-telescope: DIR := $(shell mktemp -d)
pack-telescope: pkg telescope
	cp assets/build/telescope/orbit.manifest.json $(DIR)
	mkdir -p $(DIR)/rootfs/usr/bin $(DIR)/rootfs/etc/web-assets/
	cp $(GOPATH)/bin/telescope $(DIR)/rootfs/usr/bin
	cp $(GOPATH)/bin/tscopectl $(DIR)/rootfs/usr/bin
	cp -r ./assets/web/* $(DIR)/rootfs/etc/web-assets/
	orbit pack $(DIR) $(PKG)
	rm -rf $(DIR)

pkg:
	@if [ "$$PKG" = "" ] ; then echo "ERROR: enter PKG parameter:\n\nmake publish PKG=<name>:<sem-ver>, e.g. teleport:0.0.1\n\n" && exit 255; fi

run-embedded: install
	rm -f /tmp/teleport.auth.sock
	teleport --config=examples/embedded.yaml


run-telescope: install
	rm -f /tmp/telescope.auth.sock
	telescope --config=examples/telescope.yaml

trust-telescope: 
	tscopectl user-ca pub-key > /tmp/user.pubkey
	tscopectl host-ca pub-key > /tmp/host.pubkey
	tctl remote-ca upsert --type=user --id=user.telescope.vendor.io --fqdn=telescope.vendor.io --path=/tmp/user.pubkey
	tctl remote-ca upsert --type=host --id=host.telescope.vendor.io --fqdn=telescope.vendor.io --path=/tmp/host.pubkey
	tctl remote-ca ls --type=user
	tctl remote-ca ls --type=host
	tctl host-ca pub-key > /tmp/tscope-host.pubkey
	tscopectl remote-ca upsert --type=host --id=host.auth.vendor.io --fqdn=auth.vendor.io --path=/tmp/tscope-host.pubkey
	tscopectl remote-ca ls --type=host

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
