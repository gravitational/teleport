TCD_NODE1 := http://127.0.0.1:4001
ETCD_NODES := ${ETCD_NODE1}
ETCD_FLAGS := TELEPORT_TEST_ETCD_NODES=${ETCD_NODES}

.PHONY: install test test-with-etcd remove-temp files test-package update test-grep-package cover-package cover-package-with-etcd run profile sloccount set-etcd install-assets docs-serve

install: remove-temp-files
	go install github.com/gravitational/teleport/teleport
	go install github.com/gravitational/teleport/tctl
	go install github.com/gravitational/teleport/telescope/telescope

install-assets:
	go get github.com/jteeuwen/go-bindata/go-bindata
	go install github.com/gravitational/teleport/Godeps/_workspace/src/github.com/elazarl/go-bindata-assetfs/go-bindata-assetfs
	go-bindata-assetfs -pkg="cp" ./assets/...
	mv bindata_assetfs.go ./cp
	sed -i 's|github.com/elazarl/go-bindata-assetfs|github.com/gravitational/teleport/Godeps/_workspace/src/github.com/elazarl/go-bindata-assetfs|' ./cp/bindata_assetfs.go

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

run-auth: install
	rm -f /tmp/teleport.auth.sock
	teleport\
             --log=console\
             --log-severity=INFO\
             --data-dir=/tmp\
             --fqdn=auth.gravitational.io\
	   auth\
             --backend=bolt\
             --backend-config='{"path": "/tmp/teleport.auth.db"}'\
	     --event-backend-config='{"path": "/tmp/teleport.event.db"}'\
	     --record-backend-config='{"path": "/tmp/teleport.records.db"}'\
             --domain=gravitational.io\
	     --ssh-addr=tcp://0.0.0.0:33000

run-ssh: install
	tctl token generate --output=/tmp/token --fqdn=node1.gravitational.io
	teleport\
             --log=console\
             --log-severity=INFO\
             --data-dir=/tmp\
             --fqdn=node1.gravitational.io\
             --auth-server=tcp://0.0.0.0:33000\
	   ssh\
             --token=/tmp/token\

run-cp: install #install-assets
	teleport\
             --log=console\
             --log-severity=INFO\
             --data-dir=/tmp\
             --fqdn=node2.gravitational.io\
             --auth-server=tcp://127.0.0.1:33000\
	   cp\
             --domain=gravitational.io

run-tun: install
	tctl token generate --output=/tmp/token --fqdn=node1.gravitational.io
	teleport\
             --log=console\
             --log-severity=INFO\
             --data-dir=/var/lib/teleport\
             --fqdn=node1.gravitational.io\
             --auth-server=tcp://0.0.0.0:33000\
	   tun\
             --tun-token=/tmp/token\
             --tun-srv-addr=tcp://0.0.0.0:34000\



run-embedded: install
	rm -f /tmp/teleport.auth.sock
	teleport\
             --log=console\
             --log-severity=WARN\
             --data-dir=/var/lib/teleport\
             --fqdn=auth.gravitational.io\
	   auth\
             --backend=bolt\
             --backend-config='{"path": "/var/lib/teleport/teleport.auth.db"}'\
             --domain=gravitational.io\
	     --event-backend=bolt\
             --event-backend-config='{"path": "/var/lib/teleport/teleport.event.db"}'\
	     --record-backend=bolt\
             --record-backend-config='{"path": "/var/lib/teleport/records"}'\
           ssh\
	   tun\
             --srv-addr=tcp://telescope.vendor.io:34000\
	   cp\
             --assets-dir=$(GOPATH)/src/github.com/gravitational/teleport\
             --domain=gravitational.io


run-simple: install
	rm -f /tmp/teleport.auth.sock
	mkdir -p /var/lib/teleport/records
	teleport\
             --log=console\
             --log-severity=INFO\
             --data-dir=/var/lib/teleport\
             --fqdn=auth.gravitational.io\
             --auth-server=tcp://127.0.0.1:33000\
	   auth\
             --backend=bolt\
             --backend-config='{"path": "/var/lib/teleport/teleport.auth.db"}'\
             --domain=gravitational.io\
	     --event-backend=bolt\
             --event-backend-config='{"path": "/var/lib/teleport/teleport.event.db"}'\
	     --record-backend=bolt\
             --record-backend-config='{"path": "/var/lib/teleport/records"}'\
          ssh\
	  cp\
             --assets-dir=$(GOPATH)/src/github.com/gravitational/teleport\
             --domain=gravitational.io

run-ssh2: install
	tctl token generate --output=/tmp/token --fqdn=node3.gravitational.io
	teleport\
             --log=console\
             --log-severity=INFO\
             --data-dir=/tmp\
             --fqdn=node3.gravitational.io\
             --auth-server=tcp://0.0.0.0:33000\
	   ssh\
             --addr=tcp://localhost:34001\
             --token=/tmp/token\


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
