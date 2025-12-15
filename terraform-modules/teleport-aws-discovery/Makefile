.PHONY: all
all: ;

.PHONY: fmt
fmt:
	terraform fmt -recursive .

.PHONY: docs
docs: fmt
	terraform-docs markdown table --output-file=README.md --output-mode=inject .

.PHONY: check
check:
	git ls-files -- . | xargs pre-commit run --files

.PHONY: install-pre-commit
install-pre-commit:
	pip install pre-commit
	pre-commit install
