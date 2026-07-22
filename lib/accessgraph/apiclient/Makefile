include ../../../common.mk

GENERATED_FILES = $(shell find $(abspath .) -name '*.gen.go')

.PHONY: rewrite-generated
rewrite-generated:
	cd ../../../build.assets/tooling && go run ./cmd/accessgraph-rewrite-generated
	$(MAKE) fix-imports

# The repo-wide fix-imports skips generated files; we run gci ourselves on the rewriter's output.
.PHONY: fix-imports
fix-imports:
	$(GCI) write -s standard -s default \
		-s 'prefix(github.com/gravitational/teleport)' \
		$(GENERATED_FILES)
