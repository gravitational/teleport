# Makefile for babybox related targets and variables.
#
# The babybox is a leaner, meaner, faster buildbox meant exclusively for
# codegen.
# It is not guaranteed to have tooling parity with the buildbox.
#
# See Dockerfile-babybox.

BABYBOX_BASE_NAME ?= teleport-babybox

ifeq ($(BUILDBOX_VERSION), "")
BABYBOX ?= $(BABYBOX_BASE_NAME)
else
BABYBOX ?= $(BABYBOX_BASE_NAME):$(BUILDBOX_VERSION)
endif

# BABYRUN has the necessary invocation to run a command inside the babybox.
# Use this variable to run it from other Makefiles.
BABYRUN := docker run -it --rm -v "$$(pwd)/../:/workdir" -w /workdir $(BABYBOX)

# babybox builds a codegen-focused buildbox.
# It's leaner, meaner, faster and not supposed to compile code.
.PHONY: babybox
babybox:
	DOCKER_BUILDKIT=1 docker build \
		--build-arg BUF_VERSION=$(BUF_VERSION) \
		--build-arg GOGO_PROTO_TAG=$(GOGO_PROTO_TAG) \
		--build-arg NODE_GRPC_TOOLS_VERSION=$(NODE_GRPC_TOOLS_VERSION) \
		--build-arg NODE_PROTOC_TS_VERSION=$(NODE_PROTOC_TS_VERSION) \
		--build-arg PROTOC_VER=$(PROTOC_VER) \
		--platform linux/$(RUNTIME_ARCH) \
		-f Dockerfile-babybox \
		-t "$(BABYBOX)" \
		../
