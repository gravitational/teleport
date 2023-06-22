# Those variables are extracted from build.assets/Makefile so they can be imported
# by other Makefiles
# These values may need to be updated in `dronegen/container_image_products.go` if
# they change here
BUILDBOX_VERSION ?= teleport11
BUILDBOX_BASE_NAME ?= public.ecr.aws/gravitational/teleport-buildbox

BUILDBOX=$(BUILDBOX_BASE_NAME):$(BUILDBOX_VERSION)
BUILDBOX_FIPS=$(BUILDBOX_BASE_NAME)-fips:$(BUILDBOX_VERSION)
BUILDBOX_CENTOS7=$(BUILDBOX_BASE_NAME)-centos7:$(BUILDBOX_VERSION)
BUILDBOX_CENTOS7_FIPS=$(BUILDBOX_BASE_NAME)-centos7-fips:$(BUILDBOX_VERSION)
BUILDBOX_ARM=$(BUILDBOX_BASE_NAME)-arm:$(BUILDBOX_VERSION)
BUILDBOX_ARM_FIPS=$(BUILDBOX_BASE_NAME)-arm-fips:$(BUILDBOX_VERSION)
BUILDBOX_UI=$(BUILDBOX_BASE_NAME)-ui:$(BUILDBOX_VERSION)
BUILDBOX_CONNECT=$(BUILDBOX_BASE_NAME)-connect:$(BUILDBOX_VERSION)
BUILDBOX_CENTOS7_ASSETS=$(BUILDBOX_BASE_NAME)-centos7-assets:$(BUILDBOX_VERSION)
