VERSION ?= $(shell go run ../hack/get-version/get-version.go)

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
TERRAFORM_ARCH=$(OS)_$(ARCH)

PROVIDER_PATH = ~/.terraform.d/plugins/terraform.releases.teleport.dev/gravitational/teleport/$(VERSION)/$(TERRAFORM_ARCH)/

.PHONY: install
install: build
	mkdir -p $(PROVIDER_PATH)
	mv $(BUILDDIR)/terraform-provider-teleport $(PROVIDER_PATH)
