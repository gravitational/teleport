
# build-image downloads and validates alpine artifacts, and then builds a
# docker image to be consumed by pods in Kubernetes integration tests.
.PHONY: build-image
build-image: ALPINE_VERSION ?= 3.20.3
build-image: SHORT_VERSION ?= 3.20
build-image:
	# download the alpine artifacts
	curl -fSsLO https://dl-cdn.alpinelinux.org/alpine/v$(SHORT_VERSION)/releases/x86_64/alpine-minirootfs-$(ALPINE_VERSION)-x86_64.tar.gz
	curl -fSsLO https://dl-cdn.alpinelinux.org/alpine/v$(SHORT_VERSION)/releases/x86_64/alpine-minirootfs-$(ALPINE_VERSION)-x86_64.tar.gz.asc
	curl -fSsLO https://dl-cdn.alpinelinux.org/alpine/v$(SHORT_VERSION)/releases/x86_64/alpine-minirootfs-$(ALPINE_VERSION)-x86_64.tar.gz.sha256

	# verify the checksum
	sha256sum -c alpine-minirootfs-$(ALPINE_VERSION)-x86_64.tar.gz.sha256

	# verify the signature
	gpg --import  ./alpine-ncopa.at.alpinelinux.org.asc
	gpg --verify ./alpine-minirootfs-$(ALPINE_VERSION)-x86_64.tar.gz.asc ./alpine-minirootfs-$(ALPINE_VERSION)-x86_64.tar.gz

	# build the web server
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./webserver ./webserver.go
	docker build -t alpine-webserver:v1 --build-arg=ALPINE_VERSION=$(ALPINE_VERSION) -f ./Dockerfile .

	rm webserver
	rm alpine-minirootfs-$(ALPINE_VERSION)-x86_64.tar.gz
	rm alpine-minirootfs-$(ALPINE_VERSION)-x86_64.tar.gz.asc
	rm alpine-minirootfs-$(ALPINE_VERSION)-x86_64.tar.gz.sha256


# load-image installs the custom build docker image from build-image into a KinD cluster.
.PHONY: load-image
load-image:
	kind load docker-image alpine-webserver:v1