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
	git ls-files -- . | xargs pre-commit run --files

.PHONY: install-pre-commit
install-pre-commit:
	@which pip || (echo 'pre-commit must be installed via `pip` (a Python package manager), but `pip` does not appear to be installed.' && exit 1)
	pip install pre-commit

.PHONY: enable-pre-commit
enable-pre-commit:
	pre-commit install

.PHONY: disable-pre-commit
disable-pre-commit:
	pre-commit uninstall
