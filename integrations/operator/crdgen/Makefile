build:
	cd cmd/protoc-gen-crd; go build -o protoc-gen-crd

PROTOS = \
	teleport/loginrule/v1/loginrule.proto \
	teleport/accesslist/v1/accesslist.proto \
	teleport/legacy/types/types.proto

.PHONY: build-docs
build-docs:
	cd cmd/protoc-gen-crd-docs; go build -o protoc-gen-crd-docs

.PHONY: test
test: build
# The wrappers.proto file needed for this generator exist only inside the go mod cache,
# so we retrieve the file path for the cached proto files with go mod tools.
	$(eval PROTOBUF_MOD_PATH := $(shell go mod download --json github.com/gogo/protobuf | awk -F: '/"Dir"/ { print $$2 }' | tr -d ' ",'))
	$(eval CRD_OUT_PATH := $(shell mktemp -d))

	for proto in $(PROTOS); do \
		protoc \
			-I=testdata/protofiles \
			-I=$(PROTOBUF_MOD_PATH) \
			--plugin=./protoc-gen-crd \
			--crd_out=$(CRD_OUT_PATH) \
			"$${proto}"; \
	done

	diff testdata/golden/ $(CRD_OUT_PATH);\
	EXIT_CODE=$$?;\
	rm -rf $(CRD_OUT_PATH) $(CUSTOM_IMPORTS_TMP_DIR);\
	exit $$EXIT_CODE

# Updates the input protos used for the crdgen tests. The protos are copied
# instead of imported directly avoid breaking these tests every time a proto is
# updated. The CRDs only actually change when someone manually updates them.
.PHONY: update-protos
update-protos:
	cp -r ../../../api/proto/teleport ./testdata/protofiles/

# Updates CRD snapshots used for `protoc-gen-crd` tests, they are stored in `testdata/golden`
.PHONY: update-snapshot
update-snapshot: build
	$(eval PROTOBUF_MOD_PATH := $(shell go mod download --json github.com/gogo/protobuf | awk -F: '/"Dir"/ { print $$2 }' | tr -d ' ",'))
	$(eval CRD_OUT_PATH := $(shell mktemp -d))

	for proto in $(PROTOS); do \
		protoc \
			-I=testdata/protofiles \
			-I=$(PROTOBUF_MOD_PATH) \
			--plugin=./protoc-gen-crd \
			--crd_out=$(CRD_OUT_PATH) \
			"$${proto}"; \
	done

	rm -rf testdata/golden
	cp -r $(CRD_OUT_PATH) testdata/golden
	rm -rf $(CRD_OUT_PATH)

.PHONY: debug-dump-request
debug-dump-request:
	$(eval PROTOBUF_MOD_PATH := $(shell go mod download --json github.com/gogo/protobuf | awk -F: '/"Dir"/ { print $$2 }' | tr -d ' ",'))
	for proto in $(PROTOS); do \
		protoc \
			-I=testdata/protofiles \
			-I=$(PROTOBUF_MOD_PATH) \
			--plugin=./hack/protoc-gen-dump \
			--dump_out="." \
			"$${proto}"; \
	done

