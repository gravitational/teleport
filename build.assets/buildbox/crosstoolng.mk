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
include $(mk_dir)/buildbox-common.mk

# -----------------------------------------------------------------------------
# crosstool-ng
#
# crosstool-ng is a host tool - it runs on the build host. It is installed in
# $(THIRDPARTY_HOST_PREFIX).

crosstoolng_VERSION = 1.26.0
crosstoolng_GIT_REF = crosstool-ng-$(crosstoolng_VERSION)
crosstoolng_GIT_REF_HASH = 334f6d6479096b20e80fd39e35f404319bc251b5
crosstoolng_GIT_REPO = https://github.com/crosstool-ng/crosstool-ng
crosstoolng_SRCDIR = $(call tp-src-host-dir,crosstoolng)

.PHONY: install-crosstoolng
install-crosstoolng: fetch-git-crosstoolng
	cd $(crosstoolng_SRCDIR) && ./bootstrap
	cd $(crosstoolng_SRCDIR) && ./configure --prefix=$(THIRDPARTY_HOST_PREFIX)
	$(MAKE) -C $(crosstoolng_SRCDIR) -j$(NPROC)
	$(MAKE) -C $(crosstoolng_SRCDIR) install

# -----------------------------------------------------------------------------
# Configure and build crosstool-ng compilers
#
# We use crosstool-ng, installed in $(THIRDPARTY_HOST_PREFIX) to build a
# compiler and glibc for each of the architectures: amd64, arm64, 386 and arm.
# These architecture names are as Go names them. The architecture of the
# toolchain to build is specified by the $(ARCH) variable.

CROSSTOOLNG_BUILDDIR = $(THIRDPARTY_PREFIX)/crosstoolng
$(CROSSTOOLNG_BUILDDIR):
	mkdir -p $@

CROSSTOOLNG_DEFCONFIG = $(CROSSTOOLNG_BUILDDIR)/defconfig
CROSSTOOLNG_CONFIG = $(CROSSTOOLNG_BUILDDIR)/.config

CTNG = $(THIRDPARTY_HOST_PREFIX)/bin/ct-ng -C $(CROSSTOOLNG_BUILDDIR)

# Create a defconfig if it does not exist
crosstoolng-configs/$(ARCH).defconfig:
	touch $@

# Copy the defconfig into the build dir
$(CROSSTOOLNG_DEFCONFIG): crosstoolng-configs/$(ARCH).defconfig | $(CROSSTOOLNG_BUILDDIR)
	cp $^ $@

# Create an expanded config from the defconfig
$(CROSSTOOLNG_CONFIG): $(CROSSTOOLNG_DEFCONFIG)
	$(CTNG) defconfig

# Run `ct-ng menuconfig` on the arch-specific config from the defconfig in build.assets
# and copy it back when finished with menuconfig
.PHONY: crosstoolng-menuconfig
crosstoolng-menuconfig: $(CROSSTOOLNG_CONFIG) | $(CROSSTOOLNG_BUILDDIR)
	$(CTNG) menuconfig
	$(CTNG) savedefconfig
	cp $(CROSSTOOLNG_DEFCONFIG) crosstoolng-configs/$(ARCH).defconfig

# Build the toolchain with the config in the defconfig for the architecture. We need to
# clear out some env vars because ct-ng does not want them set. We export a couple of
# vars because we reference them in the config.
# The config specifies where the toolchain is installed ($(THIRDPARTY_HOST_PREFIX)/TARGET).
.PHONY: crosstoolng-build
crosstoolng-build: $(CROSSTOOLNG_CONFIG) | $(CROSSTOOLNG_BUILDDIR)
	@mkdir -p $(THIRDPARTY_DLDIR)
	THIRDPARTY_HOST_PREFIX=$(THIRDPARTY_HOST_PREFIX) THIRDPARTY_DLDIR=$(THIRDPARTY_DLDIR) $(CTNG) build
	cd $(THIRDPARTY_HOST_PREFIX)/bin && ln -ns ../$$($(CTNG) -s show-tuple)/bin/* .
