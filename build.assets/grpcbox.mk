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

# Allow overriding the docker implementation to use for ex. podman.
DOCKER ?= docker

# GRPCBOX_RUN has the necessary invocation to run a command inside the grpcbox.
# Use this variable to run it from other Makefiles.
UID := $$(id -u)
GID := $$(id -g)
GRPCBOX_RUN := $(DOCKER) run -it --rm -u $(UID):$(GID) -v "$$(pwd)/../:/workdir" -w /workdir $(GRPCBOX)

# grpcbox builds a codegen-focused buildbox.
# It's leaner, meaner, faster and not supposed to compile code.
.PHONY: grpcbox
grpcbox:
	$(DOCKER) build \
		--build-arg BUF_VERSION=$(BUF_VERSION) \
		--build-arg UID=$(UID) --build-arg GID=$(GID) \
		-f Dockerfile-grpcbox \
		-t "$(GRPCBOX)" \
		../
