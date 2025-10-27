VERSION ?= $(shell go run ../hack/get-version/get-version.go)

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
TERRAFORM_ARCH=$(OS)_$(ARCH)

RELEASE = terraform-provider-teleportmwi-v$(VERSION)-$(OS)-$(ARCH)-bin

BUILDDIR ?= build

ADDFLAGS ?=
BUILDFLAGS ?= $(ADDFLAGS) -trimpath -ldflags '-w -s'
# CGO must NOT be enabled as hashicorp cloud does not support running providers using on CGO.
CGOFLAG ?= CGO_ENABLED=0

.PHONY: clean
clean:
	rm -rf $(BUILDDIR)/*
	rm -rf $(RELEASE).tar.gz
	go clean

.PHONY: build
build: clean
# Turning off GOWORK to prevent missing package errors.
	GOWORK=off GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -o $(BUILDDIR)/terraform-provider-teleportmwi $(BUILDFLAGS)

build-darwin-universal: $(addprefix $(BUILDDIR)/terraform-provider-teleportmwi_,arm64 amd64)
	lipo -create -output $(BUILDDIR)/terraform-provider-teleportmwi $^
	rm -r $^ $(BUILDDIR)/$(OS)

# Architecture-specific binaries for the universal binary are extracted from
# the release tarball. make will not automatically build this; you will need
# to run "make ARCH=amd64 release" and "make ARCH=arm64 release" first as is
# done in the build workflow.
$(BUILDDIR)/terraform-provider-teleportmwi_%: terraform-provider-teleportmwi-v$(VERSION)-$(OS)-%-bin.tar.gz
	mkdir -p $(BUILDDIR)/$(OS)/$*
	tar -xzf $< -C $(BUILDDIR)/$(OS)/$*
	mv $(BUILDDIR)/$(OS)/$*/terraform-provider-teleportmwi $@

# darwin-signed-build is a wrapper around the build target that ensures it is codesigned
include ../../darwin-signing.mk
.PHONY: darwin-signed-build
darwin-signed-build: BINARIES=$(BUILDDIR)/terraform-provider-teleportmwi
ifeq ($(OS)-$(ARCH),darwin-universal)
darwin-signed-build: build-darwin-universal
else
darwin-signed-build: build
endif
	$(NOTARIZE_BINARIES)

.PHONY: release
ifeq ($(OS),darwin)
release: darwin-signed-build
else
release: build
endif
	tar -C $(BUILDDIR) -czf $(RELEASE).tar.gz .

PHONY: test
test:
	go test -v -timeout=120s -parallel=10 ./...

PHONY: testacc
testacc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...

.PHONY: lint
lint:
	golangci-lint run -c ../../.golangci.yml

.PHONY: docs
docs: install
	./gen/docs.sh $(VERSION)

PROVIDER_PATH = ~/.terraform.d/plugins/terraform.releases.teleport.dev/gravitational/teleportmwi/$(VERSION)/$(TERRAFORM_ARCH)/

.PHONY: install
install: build
	mkdir -p $(PROVIDER_PATH)
	mv $(BUILDDIR)/terraform-provider-teleportmwi $(PROVIDER_PATH)
