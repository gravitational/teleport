include install.mk

LOCALDIR := $(dir $(CURDIR)/$(word $(words $(MAKEFILE_LIST)),$(MAKEFILE_LIST)))
GENTERRAFORMPATH := $(shell go env GOPATH)/bin

BUILDDIR ?= build
TFDIR ?= example

ADDFLAGS ?=
BUILDFLAGS ?= $(ADDFLAGS) -ldflags '-w -s'
CGOFLAG ?= CGO_ENABLED=0

RELEASE = terraform-provider-teleport-v$(VERSION)-$(OS)-$(ARCH)-bin

.PHONY: tfclean
tfclean:
	rm -rf $(TFDIR)/terraform.tfstate
	rm -rf $(TFDIR)/terraform.tfstate.backup
	rm -rf $(TFDIR)/.terraform
	rm -rf $(TFDIR)/.terraform.lock.hcl

.PHONY: clean
clean: tfclean
	rm -rf $(PROVIDER_PATH)*
	rm -rf $(BUILDDIR)/*
	rm -rf $(RELEASE).tar.gz
	go clean

.PHONY: build
build: clean
	GOOS=$(OS) GOARCH=$(ARCH) $(CGOFLAG) go build -o $(BUILDDIR)/terraform-provider-teleport $(BUILDFLAGS)

build-darwin-universal: $(addprefix $(BUILDDIR)/terraform-provider-teleport_,arm64 amd64)
	lipo -create -output $(BUILDDIR)/terraform-provider-teleport $^
	rm -r $^ $(BUILDDIR)/$(OS)

# Architecture-specific binaries for the universal binary are extracted from
# the release tarball. make will not automatically build this; you will need
# to run "make ARCH=amd64 release" and "make ARCH=arm64 release" first as is
# done in the build workflow.
$(BUILDDIR)/terraform-provider-teleport_%: terraform-provider-teleport-v$(VERSION)-$(OS)-%-bin.tar.gz
	mkdir -p $(BUILDDIR)/$(OS)/$*
	tar -xzf $< -C $(BUILDDIR)/$(OS)/$*
	mv $(BUILDDIR)/$(OS)/$*/terraform-provider-teleport $@

CUSTOM_IMPORTS_TMP_DIR ?= /tmp/protoc-gen-terraform/custom-imports
PROTOC_GEN_TERRAFORM_VERSION ?= v2.2.0
PROTOC_GEN_TERRAFORM_EXISTS := $(shell protoc-gen-terraform version 2>&1 >/dev/null | grep 'protoc-gen-terraform $(PROTOC_GEN_TERRAFORM_VERSION)')

.PHONY: gen-tfschema
gen-tfschema:
ifndef PROTOC_GEN_TERRAFORM_EXISTS
	@echo "protoc-gen-terraform $(PROTOC_GEN_TERRAFORM_VERSION) is not installed. Please, refer to README.md for installation instructions."
	@exit -1
endif

# The wrappers.proto file needed for this generator exist only inside the go mod cache,
# so we retrieve the file path for the cached proto files with go mod tools.
	$(eval PROTOBUF_MOD_PATH := $(shell go mod download --json github.com/gogo/protobuf | jq .Dir))

	@protoc \
		-I=../../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=$(GENTERRAFORMPATH)/protoc-gen-terraform \
		--terraform_out=config=protoc-gen-terraform-teleport.yaml:./tfschema \
		teleport/legacy/types/types.proto

	mv ./tfschema/github.com/gravitational/teleport/api/types/types_terraform.go ./tfschema/

	@protoc \
		-I=../../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=$(GENTERRAFORMPATH)/protoc-gen-terraform \
		--terraform_out=config=protoc-gen-terraform-teleport-token.yaml:./tfschema \
		teleport/legacy/types/types.proto

	mkdir -p ./tfschema/token
	mv ./tfschema/github.com/gravitational/teleport/api/types/types_terraform.go ./tfschema/token/

	@protoc \
		-I=../../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=$(GENTERRAFORMPATH)/protoc-gen-terraform \
		--terraform_out=config=protoc-gen-terraform-loginrule.yaml:./tfschema \
		teleport/loginrule/v1/loginrule.proto

	@protoc \
		-I=../../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=$(GENTERRAFORMPATH)/protoc-gen-terraform \
		--terraform_out=config=protoc-gen-terraform-devicetrust.yaml:./tfschema \
		teleport/legacy/types/device.proto

	@protoc \
		-I=../../api/proto \
		-I=$(PROTOBUF_MOD_PATH) \
		--plugin=$(GENTERRAFORMPATH)/protoc-gen-terraform \
		--terraform_out=config=protoc-gen-terraform-accesslist.yaml:./tfschema \
		teleport/accesslist/v1/accesslist.proto

	mv ./tfschema/github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1/loginrule_terraform.go ./tfschema/loginrule/v1/
	mv ./tfschema/github.com/gravitational/teleport/api/gen/proto/go/teleport/accesslist/v1/accesslist_terraform.go ./tfschema/accesslist/v1/
	mv ./tfschema/github.com/gravitational/teleport/api/types/device_terraform.go ./tfschema/devicetrust/v1/
	rm -r ./tfschema/github.com/
	@go run ./gen/main.go
	@go run ./gen/main.go docs

.PHONY: release
ifeq ($(OS)-$(ARCH),darwin-universal)
release: build-darwin-universal
else
release: build
endif
	tar -C $(BUILDDIR) -czf $(RELEASE).tar.gz .

TERRAFORM_EXISTS := $(shell terraform -version 2>/dev/null | grep 'Terraform v1.')
CURRENT_ULIMIT := $(shell ulimit -n)

TEST_ARGS?=

.PHONY: test
test: install
ifndef TERRAFORM_EXISTS
	@echo "Terraform v1.4+ is not installed (tfenv install 1.5.6 && tfenv use 1.5.6)."
	terraform -version
	@exit -1
endif
# NOTE: This is related to an old bug in Terraform and will be fixed in future releases (possibly, by a workaround on our side)
ifeq ($(shell expr $(CURRENT_ULIMIT) \< 1024), 1)
	@echo "ulimit -n is too low ($(CURRENT_ULIMIT)), please set ulimit -n 1024"
	@exit -1
endif
	go test ./testlib -v $(TEST_ARGS)

.PHONY: test-full
test-full: TEST_ARGS=--tags enterprisetests
test-full: test

.PHONY: test-ent
test-ent: TEST_ARGS=--tags enterprisetests -run 'TestTerraformEnterprise.*'
test-ent: test

.PHONY: apply
apply: install
	terraform -chdir=$(TFDIR) init && terraform -chdir=$(TFDIR) apply -auto-approve

.PHONY: reapply
reapply:
	terraform -chdir=$(TFDIR) apply

.PHONY: destroy
destroy:
	terraform -chdir=$(TFDIR) destroy -auto-approve

.PHONY: lint
lint:
	golangci-lint run -c ../../.golangci.yml
