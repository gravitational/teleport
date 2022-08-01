build:
	go build -o protoc-gen-crd

.PHONY: test
test: build
# The wrappers.proto file needed for this generator exist only inside the go mod cache,
# so we retrieve the file path for the cached proto files with go mod tools.
	$(eval PROTOBUF_MOD_PATH := $(shell go mod download --json github.com/gogo/protobuf | awk -F: '/"Dir"/ { print $$2 }' | tr -d ' ",'))
	$(eval CUSTOM_IMPORTS_TMP_DIR := $(shell mktemp -d))
	mkdir -p $(CUSTOM_IMPORTS_TMP_DIR)/github.com/gravitational/teleport/api/types/wrappers
	cp testdata/protofiles/wrappers.proto $(CUSTOM_IMPORTS_TMP_DIR)/github.com/gravitational/teleport/api/types/wrappers
	$(eval CRD_OUT_PATH := $(shell mktemp -d))
	protoc \
		-I=testdata/protofiles \
		-I=$(PROTOBUF_MOD_PATH) \
		-I=$(CUSTOM_IMPORTS_TMP_DIR) \
		--plugin=./protoc-gen-crd \
		--crd_out=$(CRD_OUT_PATH) \
		types.proto
	diff testdata/golden/ $(CRD_OUT_PATH);\
	EXIT_CODE=$$?;\
	rm -rf $(CRD_OUT_PATH) $(CUSTOM_IMPORTS_TMP_DIR);\
	exit $$EXIT_CODE