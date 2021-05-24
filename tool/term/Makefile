BUILDDIR ?= build

.PHONY: run
run:
	./$(BUILDDIR)/term

.PHONY: build
build: ui-build cli-build

.PHONY: ui-build
ui-build:
	cd ui && yarn build

.PHONY: cli-build
cli-build:
	go build -o $(BUILDDIR)/term github.com/gravitational/teleport/tool/term/term

# Installs deps
.PHONY: install
install:
	cd ui && yarn install

# buildbox builds docker buildbox image used to compile binaries and generate GRPc stuff
.PHONY: grpc-buildbox
grpc-buildbox: PROTOC_VER ?= 3.11.4
grpc-buildbox: GOGO_PROTO_VER ?= v1.1.1
grpc-buildbox: PLATFORM := linux-x86_64
grpc-buildbox:
	cd build.assets/grpc && PROTOC_VER=$(PROTOC_VER) GOGO_PROTO_VER=$(GOGO_PROTO_VER) PLATFORM=$(PLATFORM) BUILDBOX_IMG=$(BUILDBOX_IMG) force -d build.force
	img save $(BUILDBOX_IMG) | docker load

# proto generates GRPC defs from service definitions
.PHONY: grpc
grpc:
	docker run -v $(shell pwd):/go/src/github.com/gravitational/teleport \
               -v $(shell pwd):/go/src/github.com/gravitational/webapps $(BUILDBOX_IMG) \
      make -C /go/src/github.com/gravitational/teleport/tool/term grpc-in-buildbox

# proto generates GRPC stuff inside buildbox
.PHONY: grpc-in-buildbox
grpc-in-buildbox:
# standard GRPC output
	echo $$PROTO_INCLUDE
	cd proto && protoc -I=.:$$PROTO_INCLUDE \
	  --gofast_out=plugins=grpc:.\
      --plugin=protoc-gen-ts=/node_modules/.bin/protoc-gen-ts \
      --ts_out=service=grpc-web:../web/src/proto \
      --js_out=import_style=commonjs,binary:../web/src/proto \
    *.proto

