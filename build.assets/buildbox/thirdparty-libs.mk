# Third-party libraries needed to build Teleport.

mk_dir := $(dir $(lastword $(MAKEFILE_LIST)))
include $(mk_dir)/cross-compile.mk

# We build these libraries ourself and statically link them into the Teleport
# binary as we need them build with PIE (Position Independent Executable) mode
# so as to make use of ASLR (Address Space Layout Randomization). We cannot
# rely on a host OS/packager to have built them this way.

THIRDPARTY_LIBS = zlib zstd libelf libbpf libtirpc libpam libudev_zero \
		  libcbor openssl libfido2 libpcsclite

.PHONY: thirdparty-build-libs
thirdparty-build-libs: $(addprefix tp-build-,$(THIRDPARTY_LIBS))

# -----------------------------------------------------------------------------
# zlib

zlib_VERSION = 1.3.1
zlib_GIT_REF = v$(zlib_VERSION)
zlib_GIT_REF_HASH = 51b7f2abdade71cd9bb0e7a373ef2610ec6f9daf
zlib_GIT_REPO = https://github.com/madler/zlib
zlib_SRCDIR = $(call tp-src-dir,zlib)

.PHONY: tp-build-zlib
tp-build-zlib: fetch-git-zlib
	cd $(zlib_SRCDIR) && \
		./configure --prefix="$(THIRDPARTY_PREFIX)" --static
	$(MAKE) -C $(zlib_SRCDIR) CFLAGS+=-fPIE -j$(NPROC)
	$(MAKE) -C $(zlib_SRCDIR) install

# -----------------------------------------------------------------------------
# zstd

zstd_VERSION = 1.5.6
zstd_GIT_REF = v$(zstd_VERSION)
zstd_GIT_REF_HASH = 794ea1b0afca0f020f4e57b6732332231fb23c70
zstd_GIT_REPO = https://github.com/facebook/zstd
zstd_SRCDIR = $(call tp-src-dir,zstd)

.PHONY: tp-build-zstd
tp-build-zstd: fetch-git-zstd
	$(MAKE) -C $(zstd_SRCDIR) PREFIX=$(THIRDPARTY_PREFIX) CPPFLAGS_STATICLIB+=-fPIE -j$(NPROC)
	$(MAKE) -C $(zstd_SRCDIR) install PREFIX=$(THIRDPARTY_PREFIX)

# -----------------------------------------------------------------------------
# libelf

libelf_VERSION = 0.191
libelf_GIT_REF = v$(libelf_VERSION)
libelf_GIT_REF_HASH = b80c36da9d70158f9a38cfb9af9bb58a323a5796
libelf_GIT_REPO = https://github.com/arachsys/libelf
libelf_SRCDIR = $(call tp-src-dir,libelf)

.PHONY: tp-build-libelf
tp-build-libelf: fetch-git-libelf
	$(MAKE) -C $(libelf_SRCDIR) CFLAGS+=-fPIE -j$(NPROC) libelf.a
	$(MAKE) -C $(libelf_SRCDIR) install-headers install-static PREFIX=$(THIRDPARTY_PREFIX)
	sed "s|@@PREFIX@@|${THIRDPARTY_PREFIX}|" \
		< pkgconfig/libelf.pc \
		> $(PKG_CONFIG_PATH)/libelf.pc

# -----------------------------------------------------------------------------
# libbpf

libbpf_VERSION = 1.2.2
libbpf_GIT_REF = v$(libbpf_VERSION)
libbpf_GIT_REF_HASH = 1728e3e4bef0e138ea95ffe62163eb9a6ac6fa32
libbpf_GIT_REPO = https://github.com/libbpf/libbpf
libbpf_SRCDIR = $(call tp-src-dir,libbpf)

.PHONY: tp-build-libbpf
tp-build-libbpf: fetch-git-libbpf
	$(MAKE) -C $(libbpf_SRCDIR)/src \
		BUILD_STATIC_ONLY=y EXTRA_CFLAGS=-fPIE PREFIX=$(THIRDPARTY_PREFIX) LIBSUBDIR=lib V=1 \
		install install_uapi_headers

# -----------------------------------------------------------------------------
# libtirpc

libtirpc_VERSION = 1.3.4
libtirpc_SHA1 = 63c800f81f823254d2706637bab551dec176b99b
libtirpc_DOWNLOAD_URL = https://zenlayer.dl.sourceforge.net/project/libtirpc/libtirpc/$(libtirpc_VERSION)/libtirpc-$(libtirpc_VERSION).tar.bz2
libtirpc_STRIP_COMPONENTS = 1
libtirpc_SRCDIR = $(call tp-src-dir,libtirpc)

.PHONY: tp-build-libtirpc
tp-build-libtirpc: fetch-https-libtirpc
	cd $(libtirpc_SRCDIR) && \
		CFLAGS=-fPIE ./configure \
		--prefix=$(THIRDPARTY_PREFIX) \
		--enable-shared=no \
		--disable-gssapi \
		$(if $(CROSSTOOLNG_TARGET),--host=$(CROSSTOOLNG_TARGET))
	$(MAKE) -C $(libtirpc_SRCDIR) -j$(NPROC)
	$(MAKE) -C $(libtirpc_SRCDIR) install

# -----------------------------------------------------------------------------
# libpam

libpam_VERSION = 1.6.1
libpam_GIT_REF = v$(libpam_VERSION)
libpam_GIT_REF_HASH = 9438e084e2b318bf91c3912c0b8ff056e1835486
libpam_GIT_REPO = https://github.com/linux-pam/linux-pam
libpam_SRCDIR = $(call tp-src-dir,libpam)

# libpam wants the host arg to be i686 for 386 builds. The other architectures
# are just the architecture name we use.
libpam_HOST_386 = i686

.PHONY: tp-build-libpam
tp-build-libpam: fetch-git-libpam
	cd $(libpam_SRCDIR) && \
		./autogen.sh
	cd $(libpam_SRCDIR) && \
		CFLAGS=-fPIE ./configure --prefix=$(THIRDPARTY_PREFIX) \
		--disable-doc --disable-examples \
		--includedir=$(THIRDPARTY_PREFIX)/include/security \
		--host=$(or $(libpam_HOST_$(ARCH)),$(ARCH))
	$(MAKE) -C $(libpam_SRCDIR) -j$(NPROC)
	$(MAKE) -C $(libpam_SRCDIR) install

# -----------------------------------------------------------------------------
# libudev-zero

libudev_zero_VERSION = 1.0.3
libudev_zero_GIT_REF = $(libudev_zero_VERSION)
libudev_zero_GIT_REF_HASH = ee32ac5f6494047b9ece26e7a5920650cdf46655
libudev_zero_GIT_REPO = https://github.com/illiliti/libudev-zero
libudev_zero_SRCDIR = $(call tp-src-dir,libudev_zero)

.PHONY: tp-build-libudev_zero
tp-build-libudev_zero: fetch-git-libudev_zero
	$(MAKE) -C $(libudev_zero_SRCDIR) \
		PREFIX=$(THIRDPARTY_PREFIX) \
		install-static -j$(NPROC)

# -----------------------------------------------------------------------------
# libcbor

libcbor_VERSION = 0.11.0
libcbor_GIT_REF = v$(libcbor_VERSION)
libcbor_GIT_REF_HASH = 170bee2b82cdb7b2ed25af301f62cb6efdd40ec1
libcbor_GIT_REPO = https://github.com/PJK/libcbor
libcbor_SRCDIR = $(call tp-src-dir,libcbor)

.PHONY: tp-build-libcbor
tp-build-libcbor: fetch-git-libcbor
	cd $(libcbor_SRCDIR) && \
		cmake \
		-DCMAKE_INSTALL_PREFIX=$(THIRDPARTY_PREFIX) \
		-DCMAKE_POSITION_INDEPENDENT_CODE=ON \
		-DCMAKE_BUILD_TYPE=Release \
		-DWITH_EXAMPLES=OFF \
		.
	$(MAKE) -C $(libcbor_SRCDIR) -j$(NPROC)
	$(MAKE) -C $(libcbor_SRCDIR) install

# -----------------------------------------------------------------------------
# openssl

openssl_VERSION = 3.0.16
openssl_GIT_REF = openssl-$(openssl_VERSION)
openssl_GIT_REF_HASH = fa1e5dfb142bb1c26c3c38a10aafa7a095df52e5
openssl_GIT_REPO = https://github.com/openssl/openssl
openssl_SRCDIR = $(call tp-src-dir,openssl)

openssl_TARGET_linux_amd64 = linux-x86_64
openssl_TARGET_linux_arm64 = linux-aarch64
openssl_TARGET_linux_386 = linux-x86
#openssl_TARGET_linux_arm = linux-generic32
openssl_TARGET_linux_arm = linux-armv4
openssl_TARGET = $(or $(openssl_TARGET_linux_$(ARCH)),$(error Unsupported ARCH ($(ARCH)) for openssl))

.PHONY: tp-build-openssl
tp-build-openssl: fetch-git-openssl
	cd $(openssl_SRCDIR) && \
		./config "$(openssl_TARGET)" enable-fips --release -fPIC no-shared \
		--prefix=$(THIRDPARTY_PREFIX) \
		--libdir=$(THIRDPARTY_PREFIX)/lib
	$(MAKE) -C $(openssl_SRCDIR) -j$(NPROC)
	$(MAKE) -C $(openssl_SRCDIR) install_sw install_ssldirs install_fips
	sed "s|@@PREFIX@@|${THIRDPARTY_PREFIX}|" \
		< pkgconfig/libcrypto-static.pc \
		> $(PKG_CONFIG_PATH)/libcrypto-static.pc

# -----------------------------------------------------------------------------
# libfido2

libfido2_VERSION = 1.15.0
libfido2_GIT_REF = $(libfido2_VERSION)
libfido2_GIT_REF_HASH = f87c19c9487c0131531314d9ccb475ea5325794e
libfido2_GIT_REPO = https://github.com/Yubico/libfido2
libfido2_SRCDIR = $(call tp-src-dir,libfido2)

.PHONY: tp-build-libfido2
tp-build-libfido2: fetch-git-libfido2
	cd $(libfido2_SRCDIR) && \
		cmake \
		-DCMAKE_C_FLAGS="-ldl -pthread" \
		-DBUILD_SHARED_LIBS=OFF \
		-DCMAKE_INSTALL_PREFIX=$(THIRDPARTY_PREFIX) \
		-DCMAKE_POSITION_INDEPENDENT_CODE=ON \
		-DCMAKE_BUILD_TYPE=Release \
		-DBUILD_EXAMPLES=OFF \
		-DBUILD_MANPAGES=OFF \
		-DBUILD_TOOLS=OFF \
		.
	$(MAKE) -C $(libfido2_SRCDIR) -j$(NPROC)
	$(MAKE) -C $(libfido2_SRCDIR) install
	sed "s|@@PREFIX@@|${THIRDPARTY_PREFIX}|" \
		< pkgconfig/libfido2-static.pc \
		> $(PKG_CONFIG_PATH)/libfido2-static.pc

# -----------------------------------------------------------------------------
# libpcsclite
#
# Needed for PIV support in teleport and tsh

libpcsclite_VERSION = 1.9.9-teleport
libpcsclite_GIT_REF = $(libpcsclite_VERSION)
libpcsclite_GIT_REF_HASH = eb815b51504024c2218471736ba651cef147f368
libpcsclite_GIT_REPO = https://github.com/gravitational/PCSC
libpcsclite_SRCDIR = $(call tp-src-dir,libpcsclite)

.PHONY: tp-build-libpcsclite
tp-build-libpcsclite: fetch-git-libpcsclite
	cd $(libpcsclite_SRCDIR) && ./bootstrap
	cd $(libpcsclite_SRCDIR) && ./configure \
		$(if $(CROSSTOOLNG_TARGET),--target=$(CROSSTOOLNG_TARGET)) \
		$(if $(CROSSTOOLNG_TARGET),--host=$(CROSSTOOLNG_TARGET)) \
		--prefix="$(THIRDPARTY_PREFIX)" \
		--enable-static --with-pic \
		--disable-libsystemd --with-systemdsystemunitdir=no
	$(MAKE) -C $(libpcsclite_SRCDIR)/src -j$(NPROC) PROGRAMS= all
	$(MAKE) -C $(libpcsclite_SRCDIR)/src PROGRAMS= install
