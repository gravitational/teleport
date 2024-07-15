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
include $(mk_dir)/bbcommon.mk

# Environment setup for building with ctng toolchain and third party libraries.

# Define the crosstool-NG target triple
CTNG_TARGET = $(CTNG_TARGET_$(ARCH))
CTNG_TARGET_amd64 = x86_64-unknown-linux-gnu
CTNG_TARGET_arm64 = aarch64-unknown-linux-gnu
CTNG_TARGET_386 = i686-unknown-linux-gnu
CTNG_TARGET_arm = armv7-unknown-linux-gnueabi

# Define environment variables used by gcc, clang and make to find the
# appropriate compiler and third party libraries.
export C_INCLUDE_PATH = $(THIRDPARTY_PREFIX)/include
export LIBRARY_PATH = $(THIRDPARTY_PREFIX)/lib
export PKG_CONFIG_PATH = $(THIRDPARTY_PREFIX)/lib/pkgconfig
export CC = $(CTNG_TARGET)-gcc
export CXX = $(CTNG_TARGET)-g++
export LD = $(CTNG_TARGET)-ld
export PATH := $(THIRDPARTY_HOST_PREFIX)/$(CTNG_TARGET)/bin:$(PATH)

CROSS_VARS = C_INCLUDE_PATH LIBRARY_PATH PKG_CONFIG_PATH CC CXX LD PATH

.PHONY: diag-cross-vars
diag-cross-vars:
	@echo C_INCLUDE_PATH - $${C_INCLUDE_PATH}
	@echo LIBRARY_PATH - $${LIBRARY_PATH}
	@echo PKG_CONFIG_PATH - $${PKG_CONFIG_PATH}
	@echo CC - $${CC}
	@echo CXX - $${CXX}
	@echo LD - $${LD}
	@echo PATH - $${PATH}

# sh-cross-vars prints the cross-compiling variables in a form that can be
# sourced by the shell, allowing you to set them in an outer shell for
# development purposes:
# eval $(make -s -f cross-compile.sh ARCH=arm64 sh-cross-vars)
.PHONY:sh-cross-vars
sh-cross-vars:
	@/usr/bin/env bash -c 'for v in $(CROSS_VARS); do echo "export $$v=$${!v@Q}"; done'
