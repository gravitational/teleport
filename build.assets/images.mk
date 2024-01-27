# Those variables are extracted from build.assets/Makefile so they can be imported
# by other Makefiles
# These values may need to be updated in `dronegen/container_image_products.go` if
# they change here
BUILDBOX_VERSION ?= teleport15
BUILDBOX_BASE_NAME ?= ghcr.io/gravitational/teleport-buildbox

BUILDBOX = $(BUILDBOX_BASE_NAME):$(BUILDBOX_VERSION)
BUILDBOX_CENTOS7 = $(BUILDBOX_BASE_NAME)-centos7:$(BUILDBOX_VERSION)
BUILDBOX_CENTOS7_ASSETS = $(BUILDBOX_BASE_NAME)-centos7-assets:$(BUILDBOX_VERSION)
BUILDBOX_CENTOS7_FIPS = $(BUILDBOX_BASE_NAME)-centos7-fips:$(BUILDBOX_VERSION)
BUILDBOX_ARM = $(BUILDBOX_BASE_NAME)-arm:$(BUILDBOX_VERSION)
BUILDBOX_UI = $(BUILDBOX_BASE_NAME)-ui:$(BUILDBOX_VERSION)
BUILDBOX_NODE = $(BUILDBOX_BASE_NAME)-node:$(BUILDBOX_VERSION)

BUILDBOX_CENTOS7_ASSETS_SHA_amd64 = 9bbbd6c1b3d6fccff67d893b3fe3c33f9df6873c4b7b70a028e5e68ddab09a48
BUILDBOX_CENTOS7_ASSETS_SHA_arm64 = c26b01edef5a3631bd1e75a98389e5c62d33d31ae44831222484f7a45e59cbda

.PHONY:show-buildbox-base-image
show-buildbox-base-image:
	@echo "$(BUILDBOX)"

.PHONY:show-buildbox-centos7-image
show-buildbox-centos7-image:
	@echo "$(BUILDBOX_CENTOS7)"
