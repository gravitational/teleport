# This makefile fragment is included in build.assets/Makefile.
# It depends on these already being included:
#   * build.assets/version.mk (for GOLANG_VERSION AND RUST_VERSION)
#   * build.assets/images.mk (for BUILDBOX_NG and BUILDBOX_THIRDPARTY)


#
# Build the buildbox-ng using the pre-built third party components from the
# buildbox-thirdparty image
#
.PHONY: buildbox-ng
buildbox-ng:
	docker buildx build \
		--build-arg THIRDPARTY_IMAGE=$(BUILDBOX_THIRDPARTY) \
		--build-arg GOLANG_VERSION=$(GOLANG_VERSION) \
		--build-arg RUST_VERSION=$(RUST_VERSION) \
		--cache-from $(BUILDBOX_NG) \
		--cache-to type=inline \
		$(if $(PUSH),--push,--load) \
		--tag $(BUILDBOX_NG) \
		-f buildbox/Dockerfile \
		buildbox

#
# Build the buildbox thirdparty components. This rarely needs to be rebuilt and is
# slow to build, so it is done separately from the main buildbox
#
.PHONY: buildbox-thirdparty
buildbox-thirdparty:
	docker buildx build \
		--cache-from $(BUILDBOX_THIRDPARTY) \
		--cache-to type=inline \
		$(if $(PUSH),--push,--load) \
		--tag $(BUILDBOX_THIRDPARTY) \
		-f buildbox/Dockerfile-thirdparty \
		buildbox

#
# A generic build rule to build a stage of Dockerfile-thirdparty based
# on the $(STAGE) variable. These stage builds are used for development
# of the thirdparty buildbox, whether to configure crosstool-NG
# (see config/buildbox-ng), or when adding additional third party
# libraries using either the compilers stage or libs stage.
#
.PHONY: buildbox-thirdparty-stage
buildbox-thirdparty-stage:
	docker buildx build \
		--load \
		--tag buildbox-thirdparty-$(STAGE):$(BUILDBOX_VERSION) \
		-f buildbox/Dockerfile-thirdparty \
		--target $(STAGE) \
		buildbox

.PHONY: buildbox-thirdparty-crosstoolng
buildbox-thirdparty-crosstoolng: STAGE=crosstoolng
buildbox-thirdparty-crosstoolng: buildbox-thirdparty-stage

.PHONY: buildbox-thirdparty-compilers
buildbox-thirdparty-compilers: STAGE=compilers
buildbox-thirdparty-compilers: buildbox-thirdparty-stage

.PHONY: buildbox-thirdparty-libs
buildbox-thirdparty-libs: STAGE=libs
buildbox-thirdparty-libs: buildbox-thirdparty-stage

