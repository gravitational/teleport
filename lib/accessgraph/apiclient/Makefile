.PHONY: rewrite-generated
rewrite-generated:
	cd ../../../build.assets/tooling && go run ./cmd/accessgraph-rewrite-generated
	$(MAKE) -C ../../.. fix-imports
