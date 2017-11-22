PROTOC_VER ?= 3.0.0
GOGO_PROTO_TAG ?= v0.3
GRPC_GATEWAY_TAG ?= v1.1.0
PLATFORM := linux-x86_64
GRPC_API := .
BUILDBOX_TAG := reporting-buildbox:0.0.1

.PHONY: all
all: grpc build

.PHONY: build
build:
	go build ./...

.PHONY: test
test:
	go test -v ./...

.PHONY: buildbox
buildbox:
	docker build \
          --build-arg PROTOC_VER=$(PROTOC_VER) \
          --build-arg GOGO_PROTO_TAG=$(GOGO_PROTO_TAG) \
          --build-arg GRPC_GATEWAY_TAG=$(GRPC_GATEWAY_TAG) \
          --build-arg PLATFORM=$(PLATFORM) \
          -t $(BUILDBOX_TAG) .

.PHONY: grpc
grpc: buildbox
	docker run -v $(shell pwd):/go/src/github.com/gravitational/reporting $(BUILDBOX_TAG) \
		make -C /go/src/github.com/gravitational/reporting buildbox-grpc

.PHONY: buildbox-grpc
buildbox-grpc:
	echo $$PROTO_INCLUDE
	cd $(GRPC_API) && protoc -I=.:$$PROTO_INCLUDE \
      --gofast_out=plugins=grpc:.\
      --grpc-gateway_out=logtostderr=true:. \
    *.proto
