# Those variables are extracted from build.assets/Makefile so they can be imported
# by other Makefiles
ifeq ($(origin RUNTIME_ARCH), undefined)
DIR := $(dir $(lastword $(MAKEFILE_LIST)))
include $(DIR)arch.mk
endif

BUILDBOX_VERSION ?= teleport18
BUILDBOX_BASE_NAME ?= ghcr.io/gravitational/teleport-buildbox

BUILDBOX = $(BUILDBOX_BASE_NAME):$(BUILDBOX_VERSION)
BUILDBOX_CENTOS7_NOARCH = $(BUILDBOX_BASE_NAME)-centos7:$(BUILDBOX_VERSION)
BUILDBOX_CENTOS7 = $(BUILDBOX_CENTOS7_NOARCH)-$(RUNTIME_ARCH)
BUILDBOX_CENTOS7_ASSETS = $(BUILDBOX_BASE_NAME)-centos7-assets:$(BUILDBOX_VERSION)-$(RUNTIME_ARCH)
BUILDBOX_CENTOS7_FIPS = $(BUILDBOX_BASE_NAME)-centos7-fips:$(BUILDBOX_VERSION)-$(RUNTIME_ARCH)
BUILDBOX_ARM = $(BUILDBOX_BASE_NAME)-arm:$(BUILDBOX_VERSION)
BUILDBOX_UI = $(BUILDBOX_BASE_NAME)-ui:$(BUILDBOX_VERSION)
BUILDBOX_NODE = $(BUILDBOX_BASE_NAME)-node:$(BUILDBOX_VERSION)

BUILDBOX_NG = $(BUILDBOX_BASE_NAME)-ng:$(BUILDBOX_VERSION)
BUILDBOX_THIRDPARTY = $(BUILDBOX_BASE_NAME)-thirdparty:$(BUILDBOX_VERSION)

.PHONY:show-buildbox-base-image
show-buildbox-base-image:
	@echo "$(BUILDBOX)"

# show-buildbox-centos7-image is used by the spacelift-runner workflow to know
# the buildbox needed to build tbot. It needs the image without the
# architecture as it is a `docker buildx` multi-arch build so it needs to
# construct the image name itself based on $TARGETARCH not $(RUNTIME_ARCH) from
# the Makefile.
.PHONY:show-buildbox-centos7-image
show-buildbox-centos7-image:
	@echo "$(BUILDBOX_CENTOS7_NOARCH)"
