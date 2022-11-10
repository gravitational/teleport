# Those variables are extracted from build.assets/Makefile so they can be imported
# by other Makefiles
# These values may need to be updated in `dronegen/container_image_products.go` if
# they change here
BUILDBOX_VERSION ?= teleport11

BUILDBOX=public.ecr.aws/gravitational/teleport-buildbox:$(BUILDBOX_VERSION)
BUILDBOX_FIPS=public.ecr.aws/gravitational/teleport-buildbox-fips:$(BUILDBOX_VERSION)
BUILDBOX_CENTOS7=public.ecr.aws/gravitational/teleport-buildbox-centos7:$(BUILDBOX_VERSION)
BUILDBOX_CENTOS7_FIPS=public.ecr.aws/gravitational/teleport-buildbox-centos7-fips:$(BUILDBOX_VERSION)
BUILDBOX_ARM=public.ecr.aws/gravitational/teleport-buildbox-arm:$(BUILDBOX_VERSION)
BUILDBOX_ARM_FIPS=public.ecr.aws/gravitational/teleport-buildbox-arm-fips:$(BUILDBOX_VERSION)
BUILDBOX_TELETERM=public.ecr.aws/gravitational/teleport-buildbox-teleterm:$(BUILDBOX_VERSION)
