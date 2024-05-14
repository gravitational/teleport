VERSION?=15.2.0
GO_VERSION=1.21.9

GITTAG=v$(VERSION)

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
TERRAFORM_ARCH=$(OS)_$(ARCH)

LOCALDIR := $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST)))
GENTERRAFORMPATH := $(shell go env GOPATH)/bin

BUILDDIR ?= build
BINARY = $(BUILDDIR)/teleport-event-handler

GITREF ?= $(shell git describe --dirty --long --tags --match '*event-handler*')
ADDFLAGS ?=
BUILDFLAGS ?= $(ADDFLAGS) -ldflags "-w -s -X main.Gitref=$(GITREF) -X main.Version=$(VERSION)"
CGOFLAG ?= CGO_ENABLED=0

PASS ?= 1234

KEYLEN = 1024

CLOUD_ADDR=evilmartians.teleport.sh:443
IDENTITY_FILE=example/keys/identity

DOCKER_VERSION = $(subst +,_,$(VERSION))
DOCKER_NAME=teleport-plugin-event-handler
DOCKER_PRIVATE_REGISTRY = 146628656107.dkr.ecr.us-west-2.amazonaws.com
DOCKER_IMAGE_BASE = $(DOCKER_PRIVATE_REGISTRY)/gravitational
DOCKER_IMAGE = $(DOCKER_IMAGE_BASE)/$(DOCKER_NAME):$(DOCKER_VERSION)
DOCKER_ECR_PUBLIC_REGISTRY = public.ecr.aws/gravitational
DOCKER_IMAGE_ECR_PUBLIC = $(DOCKER_ECR_PUBLIC_REGISTRY)/$(DOCKER_NAME):$(DOCKER_VERSION)
DOCKER_BUILD_ARGS = --build-arg GO_VERSION=${GO_VERSION} --build-arg GITREF=$(GITREF) --build-arg VERSION=$(VERSION)

RELEASE_NAME = teleport-event-handler
RELEASE = $(RELEASE_NAME)-v$(VERSION)-$(OS)-$(ARCH)-bin
RELEASE_MESSAGE = "Building with GOOS=$(OS) GOARCH=$(ARCH)."

.PHONY: build
build: clean
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -o $(BUILDDIR)/teleport-event-handler $(BUILDFLAGS)

.PHONY: release
release: build
	@echo "---> $(RELEASE_MESSAGE)"
	mkdir $(RELEASE_NAME)
	cp -rf $(BINARY) \
          README.md \
          install \
          $(RELEASE_NAME)/
	echo $(GITTAG) > $(RELEASE_NAME)/VERSION
	tar -czf $(RELEASE).tar.gz $(RELEASE_NAME)
	rm -rf $(RELEASE_NAME)/
	@echo "---> Created $(RELEASE).tar.gz."
  

.PHONY: clean
clean:
	go clean

.PHONY: docker-build
docker-build: ## Build docker image with the plugin.
	docker build ${DOCKER_BUILD_ARGS} -t ${DOCKER_IMAGE} -f ./Dockerfile ../

.PHONY: docker-push
docker-push:
	docker push ${DOCKER_IMAGE}

.PHONY: docker-promote
docker-promote: docker-promote-ecr-public

.PHONY: docker-promote-ecr-public
docker-promote-ecr-public:
	docker pull ${DOCKER_IMAGE} && \
	docker tag ${DOCKER_IMAGE} ${DOCKER_IMAGE_ECR_PUBLIC} && \
	docker push ${DOCKER_IMAGE_ECR_PUBLIC}

.PHONY: install
install: build
	go install

.PHONY: test
test:
	go test -coverprofile=cover.out

.PHONY: configure
configure: build
	rm -rf tmp/*
	./build/teleport-event-handler configure tmp
	tctl create -f tmp/teleport-event-handler-role.yaml
	tctl auth sign --out tmp/identity --user teleport-event-handler

.PHONY: fluentd
fluentd:
	docker run -p 8888:8888 -v $(LOCALDIR)tmp:/keys -v $(LOCALDIR)tmp/fluent.conf:/fluentd/etc/fluent.conf fluent/fluentd:edge 

.PHONY: example
example: build
	./build/teleport-event-handler start --config tmp/teleport-event-handler.toml --start-time 2020-01-01T00:00:00Z

.PHONY: gen-example-mtls
gen-example-mtls:
	mkdir -p example/keys
	rm -f example/keys/*

	openssl genrsa -out example/keys/ca.key $(KEYLEN)
	chmod 444 example/keys/ca.key
	openssl req -config example/ssl.conf -key example/keys/ca.key -new -x509 -days 7300 -sha256 -extensions v3_ca -subj "/CN=ca" -out example/keys/ca.crt

	openssl genrsa -aes256 -passout pass:$(PASS) -out example/keys/server.key $(KEYLEN)
	chmod 444 example/keys/server.key
	openssl req -passin pass:$(PASS) -config example/ssl.conf -subj "/CN=server" -key example/keys/server.key -new -out example/keys/server.csr
	openssl x509 -req -in example/keys/server.csr -CA example/keys/ca.crt -CAkey example/keys/ca.key -CAcreateserial -days 365 -out example/keys/server.crt -extfile example/ssl.conf -extensions server_cert

	openssl genrsa -out example/keys/server_nopass.key $(KEYLEN)
	chmod 444 example/keys/server_nopass.key
	openssl req -config example/ssl.conf -subj "/CN=server-nopass" -key example/keys/server_nopass.key -new -out example/keys/server_nopass.csr
	openssl x509 -req -in example/keys/server_nopass.csr -CA example/keys/ca.crt -CAkey example/keys/ca.key -CAcreateserial -days 365 -out example/keys/server_nopass.crt -extfile example/ssl.conf -extensions server_cert

	openssl genrsa -out example/keys/client.key $(KEYLEN)
	chmod 444 example/keys/client.key
	openssl req -config example/ssl.conf -subj "/CN=client" -key example/keys/client.key -new -out example/keys/client.csr
	openssl x509 -req -in example/keys/client.csr -CA example/keys/ca.crt -CAkey example/keys/ca.key -CAcreateserial -days 365 -out example/keys/client.crt -extfile example/ssl.conf -extensions client_cert

.PHONY: lint
lint:
	golangci-lint run -c ../../.golangci.yml
