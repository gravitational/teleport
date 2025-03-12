# This Makefile fragment defines some environment variables for using the
# crosstool-NG cross compilers installed in the buildbox. By including this
# makefile fragment in another Makefile, various environment variables will
# be set to find the cross compilers and the third-party libraries.
#
# $(ARCH) should be set to one of amd64, arm64, 386 or arm so that the
# correct compiler and libraries are referenced.
#
# Care should be taked to NOT include this file when building the cross
# compilers themselves.

mk_dir := $(dir $(lastword $(MAKEFILE_LIST)))
include $(mk_dir)/buildbox-common.mk

# Environment setup for building with crosstoolng toolchain and third party libraries.

# Define the crosstool-NG target triple
CROSSTOOLNG_TARGET = $(CROSSTOOLNG_TARGET_$(ARCH))
CROSSTOOLNG_TARGET_amd64 = x86_64-unknown-linux-gnu
CROSSTOOLNG_TARGET_arm64 = aarch64-unknown-linux-gnu
CROSSTOOLNG_TARGET_386 = i686-unknown-linux-gnu
CROSSTOOLNG_TARGET_arm = arm-unknown-linux-gnueabihf

# Define some vars that locate the installation of the toolchain.
CROSSTOOLNG_TOOLCHAIN = $(THIRDPARTY_HOST_PREFIX)/$(CROSSTOOLNG_TARGET)
CROSSTOOLNG_SYSROOT = $(CROSSTOOLNG_TOOLCHAIN)/$(CROSSTOOLNG_TARGET)/sysroot

# Define environment variables used by gcc, clang and make to find the
# appropriate compiler and third party libraries.
export C_INCLUDE_PATH = $(THIRDPARTY_PREFIX)/include
export LIBRARY_PATH = $(THIRDPARTY_PREFIX)/lib
export PKG_CONFIG_PATH = $(THIRDPARTY_PREFIX)/lib/pkgconfig
export CC = $(CROSSTOOLNG_TARGET)-gcc
export CXX = $(CROSSTOOLNG_TARGET)-g++
export LD = $(CROSSTOOLNG_TARGET)-ld

CROSS_VARS = C_INCLUDE_PATH LIBRARY_PATH PKG_CONFIG_PATH CC CXX LD

# Clang needs to find the gcc toolchain libraries that are not in the sysroot.
# These extra args are used by the clang-12.sh front-end script so clang is
# always invoked with the correct location for the GCC cross toolchain.
# This is used for the boring-rs crate to build boringssl in FIPS mode.
export CLANG_EXTRA_ARGS = --gcc-toolchain=$(CROSSTOOLNG_TOOLCHAIN) --sysroot=$(CROSSTOOLNG_SYSROOT)

CROSS_VARS += CLANG_EXTRA_ARGS

# arm64 has linking issues using the binutils linker when building the
# Enterprise Teleport binary ("relocation truncated to fit: R_AARCH64_CALL26
# against symbol") that is resolved by using the gold linker. Ensure that
# is used for arm64 builds
ifeq ($(ARCH),arm64)
export CTNG_LD_IS := gold
CROSS_VARS += CTNG_LD_IS
endif

# sh-cross-vars prints the cross-compiling variables in a form that can be
# sourced by the shell, allowing you to set them in an outer shell for
# development purposes:
# eval $(make -s -f cross-compile.sh ARCH=arm64 sh-cross-vars)
.PHONY:sh-cross-vars
sh-cross-vars:
	@/usr/bin/env bash -c 'for v in $(CROSS_VARS); do echo "export $$v=$${!v@Q}"; done'
