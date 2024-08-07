# =============================================================================
# FIPS builds require specific certified versions of software for both the
# compiler and the crypto itself. For the crypto, we use boringssl, but that
# must be compiled by a specific version of clang.

# Default ARCH to the host architecture. Normally it is set to specify which
# architecture to build for, but some rules need it to be set even if not used.
UNAME_M := $(shell uname -m)
ARCH_aarch64 = arm64
ARCH = $(or $(ARCH_$(UNAME_M)),$(UNAME_M))

mk_dir := $(dir $(lastword $(MAKEFILE_LIST)))
include $(mk_dir)/buildbox-common.mk

# clang-12
#
# We need to build clang-12 ourselves because we need a specific version (12.0.0)
# for FIPS compliance. That version of clang is needed to build boringssl
# for use by rust to build rdp-client (via the boring-sys crate).
#
# clang is built for the host system, using the host system compiler, not ctng.

clang_VERSION = 12.0.0
clang_GIT_REF = llvmorg-$(clang_VERSION)
clang_GIT_REF_HASH = d28af7c654d8db0b68c175db5ce212d74fb5e9bc
clang_GIT_REPO = https://github.com/llvm/llvm-project.git
clang_SRCDIR = $(call tp-src-host-dir,clang)

.PHONY: fetch-clang configure-clang install-clang
fetch-clang: fetch-git-clang
configure-clang: fetch-clang
	cd $(clang_SRCDIR) && cmake \
		-DCMAKE_BUILD_TYPE=Release \
		-DCMAKE_INSTALL_PREFIX=$(THIRDPARTY_HOST_PREFIX) \
		-DLLVM_ENABLE_PROJECTS=clang \
		-DLLVM_BUILD_TOOLS=ON \
		-G "Unix Makefiles" llvm
install-clang: configure-clang
	cd $(clang_SRCDIR) && \
		$(MAKE) -j$(NPROC) \
			install-llvm-strip \
			install-clang-format \
			install-clang \
			install-clang-resource-headers \
			install-libclang
	ln -nsf clang $(THIRDPARTY_HOST_PREFIX)/bin/clang++-12
