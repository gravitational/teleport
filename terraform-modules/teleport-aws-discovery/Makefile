.PHONY: all
all: docs;

.PHONY: fmt
fmt:
	terraform fmt -recursive .

.PHONY: docs
docs: fmt
	terraform-docs markdown table --output-file=README.md --output-mode=inject .
