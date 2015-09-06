.PHONY: install test remove-temp-files test-package test-grep-package cover-package sloccount init-trust

install: remove-temp-files
	go install github.com/gravitational/teleport/telescope/telescope/
	go install github.com/gravitational/teleport/telescope/telescope/tscopectl

test: install
	go test -v ./telescope/... -cover

remove-temp-files:
	find . -name flymake_* -delete

test-package: remove-temp-files
	go test -v ./telescope/$(p)


test-grep-package: remove-temp-files
	go test -v ./telescope/$(p) -check.f=$(e)

cover-package: remove-temp-files
	go test -v ./telescope/$(p)  -coverprofile=/tmp/coverage.out
	go tool cover -html=/tmp/coverage.out

run: install
	rm -f /tmp/telescope.auth.sock
	telescope --log=console\
         --log-severity=INFO\
         --data-dir=/var/lib/teleport/telescope\
         --assets-dir=$(GOPATH)/src/github.com/gravitational/teleport/telescope\
         --cp-assets-dir=$(GOPATH)/src/github.com/gravitational/teleport\
         --fqdn=telescope.vendor.io\
         --domain=vendor.io\
         --tun-addr=tcp://telescope.vendor.io:34000\
         --web-addr=tcp://telescope.vendor.io:35000\
         --backend=bolt\
         --backend-config='{"path": "/var/lib/teleport/telescope.db"}'


init-trust:
	tscopectl user-ca pub-key > /tmp/user.pubkey
	tscopectl host-ca pub-key > /tmp/host.pubkey
	tctl remote-ca upsert --type=user --id=user.telescope.vendor.io --fqdn=telescope.vendor.io --path=/tmp/user.pubkey
	tctl remote-ca upsert --type=host --id=host.telescope.vendor.io --fqdn=telescope.vendor.io --path=/tmp/host.pubkey
	tctl remote-ca ls --type=user
	tctl remote-ca ls --type=host

sloccount:
	find . -path ./Godeps -prune -o -name "*.go" -print0 | xargs -0 wc -l
