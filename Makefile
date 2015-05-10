ETCD_NODE1 := http://127.0.0.1:4001
ETCD_NODES := ${ETCD_NODE1}
ETCD_FLAGS := TELEPORT_TEST_ETCD_NODES=${ETCD_NODES}

.PHONY: install test test-with-etcd remove-temp files test-package update test-grep-package cover-package cover-package-with-etcd run profile sloccount set-etcd install-assets

install: remove-temp-files install-assets
	go get github.com/jteeuwen/go-bindata/go-bindata
	go install github.com/gravitational/teleport/Godeps/_workspace/src/github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs
	go-bindata-assetfs -pkg="cp" ./assets/...
	mv bindata_assetfs.go ./cp
	sed -i 's|github.com/elazarl/go-bindata-assetfs|github.com/gravitational/teleport/Godeps/_workspace/src/github.com/elazarl/go-bindata-assetfs|' ./cp/bindata_assetfs.go
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

run-auth:
	go install github.com/gravitational/teleport/teleport
	teleport -auth\
             -authBackend=etcd\
             -authBackendConfig='{"nodes": ["${ETCD_NODE1}"], "key": "/teleport"}'\
             -authDomain=gravitational.io\
             -log=console\
             -logSeverity=INFO\
             -dataDir=/tmp\
             -fqdn=auth.gravitational.io
run-ssh:
	go install github.com/gravitational/teleport/teleport
	teleport -ssh\
             -log=console\
             -logSeverity=INFO\
             -dataDir=/tmp\
             -fqdn=node1.gravitational.io\
             -authServer=tcp://auth.gravitational.io:33001\
             -sshToken=token

run-cp: install-assets
	go install github.com/gravitational/teleport/teleport
	teleport -cp\
             -cpDomain=gravitational.io\
             -log=console\
             -logSeverity=INFO\
             -dataDir=/tmp\
             -fqdn=node2.gravitational.io\
             -authServer=tcp://auth.gravitational.io:33001

profile:
	go tool pprof http://localhost:6060/debug/pprof/profile

sloccount:
	find . -path ./Godeps -prune -o -name "*.go" -print0 | xargs -0 wc -l
