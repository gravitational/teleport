# Makefile for building cross-compilers with crosstool-ng and building
# the third-party C library dependencies for Teleport.
#
# This Makefile is intended to be used inside a docker build and as such is
# Linux-only. It could be run on a linux host outside of docker, in which case
# you will likely want to override THIRDPARTY_DIR.

# Default ARCH to the host architecture. Normally it is set to specify which
# architecture to build for, but some rules need it to be set even if not used.
UNAME_M := $(shell uname -m)
ARCH_aarch64 = arm64
ARCH = $(or $(ARCH_$(UNAME_M)),$(UNAME_M))

mk_dir := $(dir $(lastword $(MAKEFILE_LIST)))
include $(mk_dir)/bbcommon.mk

# -----------------------------------------------------------------------------
# crosstool-ng
#
# crosstool-ng is a host tool - it runs on the build host. It is installed in
# $(THIRDPARTY_HOST_PREFIX).

ctng_VERSION = 1.26.0
ctng_GIT_REF = crosstool-ng-$(ctng_VERSION)
ctng_GIT_REF_HASH = 334f6d6479096b20e80fd39e35f404319bc251b5
ctng_GIT_REPO = https://github.com/crosstool-ng/crosstool-ng
ctng_SRCDIR = $(call tp-src-host-dir,ctng)

.PHONY: install-ctng
install-ctng: fetch-git-ctng
	cd $(ctng_SRCDIR) && ./bootstrap
	cd $(ctng_SRCDIR) && ./configure --prefix=$(THIRDPARTY_HOST_PREFIX)
	$(MAKE) -C $(ctng_SRCDIR) -j$(NPROC)
	$(MAKE) -C $(ctng_SRCDIR) install

# -----------------------------------------------------------------------------
# Configure and build crosstool-ng compilers
#
# We use crosstool-ng, installed in $(THIRDPARTY_HOST_PREFIX) to build a
# compiler and glibc for each of the architectures: amd64, arm64, 386 and arm.
# These architecture names are as Go names them. The architecture of the
# toolchain to build is specified by the $(ARCH) variable.

CTNG_BUILDDIR = $(THIRDPARTY_PREFIX)/ctng
$(CTNG_BUILDDIR):
	mkdir -p $@

CTNG_DEFCONFIG = $(CTNG_BUILDDIR)/defconfig
CTNG_CONFIG = $(CTNG_BUILDDIR)/.config

# Create a defconfig if it does not exist
ct-ng-configs/$(ARCH).defconfig:
	touch $@

# Copy the defconfig into the build dir
$(CTNG_DEFCONFIG): ct-ng-configs/$(ARCH).defconfig | $(CTNG_BUILDDIR)
	cp $^ $@

# Create an expanded config from the defconfig
$(CTNG_CONFIG): $(CTNG_DEFCONFIG)
	cd $(CTNG_BUILDDIR) && $(THIRDPARTY_HOST_PREFIX)/bin/ct-ng defconfig

# Run `ct-ng menuconfig` on the arch-specific config from the defconfig in build.assets
# and copy it back when finished with menuconfig
.PHONY: ctng-menuconfig
ctng-menuconfig: $(CTNG_CONFIG) | $(CTNG_BUILDDIR)
	cd $(CTNG_BUILDDIR) && $(THIRDPARTY_HOST_PREFIX)/bin/ct-ng menuconfig
	cd $(CTNG_BUILDDIR) && $(THIRDPARTY_HOST_PREFIX)/bin/ct-ng savedefconfig
	cp $(CTNG_BUILDDIR)/defconfig ct-ng-configs/$(ARCH).defconfig

# Build the toolchain with the config in the defconfig for the architecture. We need to
# clear out some env vars because ct-ng does not want them set. We export a couple of
# vars because we reference them in the config.
# The config specifies where the toolchain is installed ($(THIRDPARTY_HOST_PREFIX)/TARGET).
.PHONY: ctng-build
ctng-build: $(CTNG_CONFIG) | $(CTNG_BUILDDIR)
	@mkdir -p $(THIRDPARTY_DLDIR)
	cd $(CTNG_BUILDDIR) && \
		THIRDPARTY_HOST_PREFIX=$(THIRDPARTY_HOST_PREFIX) \
		THIRDPARTY_DLDIR=$(THIRDPARTY_DLDIR) \
		$(THIRDPARTY_HOST_PREFIX)/bin/ct-ng build
