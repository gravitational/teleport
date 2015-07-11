ETCD_NODE1 := http://127.0.0.1:4001
ETCD_NODES := ${ETCD_NODE1}
ETCD_FLAGS := TELEPORT_TEST_ETCD_NODES=${ETCD_NODES}

.PHONY: install test test-with-etcd remove-temp files test-package update test-grep-package cover-package cover-package-with-etcd run profile sloccount set-etcd install-assets docs-serve

install: remove-temp-files
	go install github.com/gravitational/teleport/teleport
	go install github.com/gravitational/teleport/tctl

install-assets:
	go get github.com/jteeuwen/go-bindata/go-bindata
	go install github.com/gravitational/teleport/Godeps/_workspace/src/github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs
	go-bindata-assetfs -pkg="cp" ./assets/...
	mv bindata_assetfs.go ./cp
	sed -i 's|github.com/elazarl/go-bindata-assetfs|github.com/gravitational/teleport/Godeps/_workspace/src/github.com/elazarl/go-bindata-assetfs|' ./cp/bindata_assetfs.go

test: remove-temp-files
	go test -v ./... -cover

test-with-etcd: remove-temp-files
	${ETCD_FLAGS} go test -v ./... -cover

remove-temp-files:
	find . -name flymake_* -delete

test-package: remove-temp-files
	go test -v ./$(p)

test-package-with-etcd: remove-temp-files
	${ETCD_FLAGS} go test -v ./$(p)

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

run-auth: install
	rm -f /tmp/teleport.auth.sock
	teleport -auth\
             -authBackend=bolt\
             -authBackendConfig='{"path": "/tmp/teleport.auth.db"}'\
             -authDomain=gravitational.io\
             -log=console\
             -logSeverity=INFO\
             -dataDir=/tmp\
             -fqdn=auth.gravitational.io

run-ssh: install
	tctl token generate --output=/tmp/token -fqdn=node1.gravitational.io
	teleport -ssh\
             -log=console\
             -logSeverity=INFO\
             -dataDir=/tmp\
             -fqdn=node1.gravitational.io\
             -sshToken=/tmp/token\
             -authServer=tcp://auth.gravitational.io:33000

run-cp: install install-assets
	teleport -cp\
             -cpDomain=gravitational.io\
             -log=console\
             -logSeverity=INFO\
             -dataDir=/tmp\
             -fqdn=node2.gravitational.io\
             -authServer=tcp://auth.gravitational.io:33000

run-tun: install
	tctl token generate --output=/tmp/token -fqdn=node1.gravitational.io
	teleport -tun\
             -log=console\
             -logSeverity=INFO\
             -dataDir=/tmp\
             -fqdn=node1.gravitational.io\
             -tunToken=/tmp/token\
             -tunSrvAddr=tcp://lens.gravitational.io:34000\
             -authServer=tcp://auth.gravitational.io:33000


run-embedded: install
	rm -f /tmp/teleport.auth.sock
	teleport -auth\
             -authBackend=bolt\
             -authBackendConfig='{"path": "/tmp/teleport.auth.db"}'\
             -authDomain=gravitational.io\
             -log=console\
             -logSeverity=INFO\
             -dataDir=/tmp\
             -fqdn=auth.gravitational.io\
             -ssh\
             -authServer=tcp://auth.gravitational.io:33000\
	         -tun\
             -tunSrvAddr=tcp://lens.vendor.io:34000


run-simple: install
	rm -f /tmp/teleport.auth.sock
	mkdir -p /var/lib/teleport/records
	teleport -auth\
             -authBackend=bolt\
             -authBackendConfig='{"path": "/var/lib/teleport/teleport.auth.db"}'\
             -authDomain=gravitational.io\
	         -authEventBackend=bolt\
             -authEventBackendConfig='{"path": "/var/lib/teleport/teleport.event.db"}'\
	         -authRecordBackend=bolt\
             -authRecordBackendConfig='{"path": "/var/lib/teleport/records"}'\
             -log=console\
             -logSeverity=INFO\
             -dataDir=/var/lib/teleport\
             -fqdn=auth.gravitational.io\
             -ssh\
             -authServer=tcp://auth.gravitational.io:33000\
			 -cp\
             -cpAssetsDir=$(GOPATH)/src/github.com/gravitational/teleport\
             -cpDomain=gravitational.io

run-ssh2: install
	tctl token generate --output=/tmp/token -fqdn=node1.gravitational.io
	teleport -ssh\
             -sshAddr=tcp://localhost:34001\
             -log=console\
             -logSeverity=INFO\
             -dataDir=/tmp\
             -fqdn=node1.gravitational.io\
             -sshToken=/tmp/token\
             -authServer=tcp://auth.gravitational.io:33000


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
