default: fmt lint install generate

VERSION ?= $(shell go run ../hack/get-version/get-version.go)

OS ?= $(shell go env GOOS)
ARCH ?= $(shell go env GOARCH)
TERRAFORM_ARCH=$(OS)_$(ARCH)

PROVIDER_PATH = ~/.terraform.d/plugins/terraform.releases.teleport.dev/gravitational/machineid/$(VERSION)/$(TERRAFORM_ARCH)/

.PHONY: install
install:
	mkdir -p $(PROVIDER_PATH)
	go build -o $(PROVIDER_PATH)/terraform-provider-machineid


build:
	go build -v ./...

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc:
	TF_ACC=1 go test -v -cover -timeout 120m ./...

.PHONY: fmt lint test testacc build install generate
