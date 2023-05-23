# Makefile for grpcbox targets and variables.
#
# The grpcbox is a leaner, meaner, faster buildbox meant exclusively for
# codegen.
# It is not guaranteed to have tooling parity with the buildbox.
#
# See Dockerfile-grpcbox.

GRPCBOX_BASE_NAME ?= teleport-grpcbox

ifeq ($(BUILDBOX_VERSION), "")
GRPCBOX ?= $(GRPCBOX_BASE_NAME)
else
GRPCBOX ?= $(GRPCBOX_BASE_NAME):$(BUILDBOX_VERSION)
endif

# UID and GID are used to run the grpcbox as the current user.
UID := $$(id -u)
GID := $$(id -g)

# GRPCBOX_RUN has the necessary invocation to run a command inside the grpcbox.
# Use this variable to run it from other Makefiles.
# Set XDG_CACHE_HOME to let non-root user cache dependencies: https://github.com/bufbuild/buf/blob/f38ba9455c6650947b0b710327f9acb708da1196/private/pkg/app/app_unix.go#LL70C18-L70C32
GRPCBOX_RUN := docker run -it --rm -u $(UID):$(GID) -e XDG_CACHE_HOME="/tmp/.cache" -v "$$(pwd)/../:/workdir" -w /workdir $(GRPCBOX)

# grpcbox builds a codegen-focused buildbox.
# It's leaner, meaner, faster and not supposed to compile code.
.PHONY: grpcbox
grpcbox:
	DOCKER_BUILDKIT=1 docker build \
		--build-arg BUF_VERSION=$(BUF_VERSION) \
		--build-arg GOGO_PROTO_TAG=$(GOGO_PROTO_TAG) \
		--build-arg NODE_GRPC_TOOLS_VERSION=$(NODE_GRPC_TOOLS_VERSION) \
		--build-arg NODE_PROTOC_TS_VERSION=$(NODE_PROTOC_TS_VERSION) \
		--build-arg PROTOC_VER=$(PROTOC_VER) \
		-f Dockerfile-grpcbox \
		-t "$(GRPCBOX)" \
		../
