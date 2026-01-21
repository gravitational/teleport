.PHONY: all
all: ;

# this is defined because Make does not support .PHONY pattern recipes, but we
# can make a no-op .PHONY recipe that a pattern recipe depends on to achieve the
# same effect
.PHONY: force
force: ;

.PHONY: fmt
fmt:
	terraform fmt -recursive .

MODULE_DIRS := $(shell find . -type f -name '*.tf' -exec dirname {} \; 2>/dev/null | sort -u)
README_FILES := $(addsuffix /README.md,$(MODULE_DIRS))

.PHONY: docs
docs: $(README_FILES) ;

%/README.md: force
	terraform-docs markdown table --config .terraform-docs.yml $*

.PHONY: check
check:
	git ls-files -- . \
	| xargs pre-commit run --hook-stage=manual --verbose --files

.PHONY: require-pre-commit
require-pre-commit:
	@which pre-commit 1>/dev/null 2>&1 || (echo 'pre-commit is not installed. For installation instructions see https://pre-commit.com/#install'; exit 1)

.PHONY: enable-pre-commit
enable-pre-commit: require-pre-commit
	pre-commit install

.PHONY: disable-pre-commit
disable-pre-commit: require-pre-commit
	pre-commit uninstall

VERSION ?= $(shell go run ../hack/get-version/get-version.go)
BUILD_DIR ?= build

# release builds the published module tarballs and then renames each tarball to innclude a version tag suffix, e.g.,
# from: build/terraform-module_<namespace>_<name>_<system>.tar.gz
# to:   build/terraform-module_<namespace>_<name>_<system>_<version>.tar.gz
.PHONY: release
release: build
	@find $(BUILD_DIR) -maxdepth 1 -name '*.tar.gz' -type f -exec bash -c '\
		for f; do \
			mv "$$f" "$${f%.tar.gz}_v$(VERSION).tar.gz"; \
		done \
	' bash {} \+

PLUGIN_TYPE := terraform-module
# This list controls which modules are actually published.
PUBLISHED_TF_MODULES := $(subst /,_, \
	teleport/discovery/aws \
)
MODULE_TARBALLS:=$(addsuffix .tar.gz, $(PUBLISHED_TF_MODULES))
BUILD_TARGETS:=$(addprefix $(BUILD_DIR)/$(PLUGIN_TYPE)_, $(MODULE_TARBALLS))

.PHONY: build
build: $(BUILD_TARGETS) ;

# Build each published module in build/terraform-module_<namespace>_<name>_<system>.tar.gz
# Module names follow the naming scheme as described in Terraform Module Registry Protocol.
# <namespace> is the org (should be "teleport")
# <name> is the module name
# <system> is the target remote system, typically a cloud provider e.g., "aws", "google", "azurerm"
# The subdirectory tree for modules follows the same naming convention, with
# a directory for each of namespace, name, and system:
# <namespace>/<name>/<system>/*.tf
# For example, if we have a module at ./teleport/discovery/aws and we are
# releasing version 19.0.0, then this will make build/terraform-module_teleport_discovery_aws_v19.0.0.tar.gz
#
# https://developer.hashicorp.com/terraform/internals/module-registry-protocol
$(BUILD_DIR)/$(PLUGIN_TYPE)_%.tar.gz: clean
	@mkdir -p $(dir $@)
	tar -C $(subst _,/,$*) -czf $@ . # change dirs to make the tarball because Terraform expects the tarball to only contain module source files, i.e., not nested in subdirectories.

.PHONY: clean
clean: tfclean
	rm -rf build/

.PHONY: tfclean
tfclean:
	find . \( \
		-name '.terraform' \
		-or -name '.terraform.lock.hcl' \
		-or -name 'terraform.tfstate*' \
	\) -exec rm -rf {} \+
