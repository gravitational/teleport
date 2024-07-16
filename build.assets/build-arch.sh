#!/bin/bash

set -e

export GOLANG_VERSION=go1.22.5

export RUST_VERSION=1.79.0

# Switch depending on the architecture which is passed as the script argument
case $1 in
    "arm64")
        export ARCH="arm64"
        export GO_ARCH="arm64"
        export SYSROOT="${HOME}/x-tools/aarch64-unknown-linux-gnu/aarch64-unknown-linux-gnu/sysroot"

        export PATH="${HOME}/x-tools/aarch64-unknown-linux-gnu/bin:$PATH"
        export CC="aarch64-unknown-linux-gnu-cc --sysroot=${SYSROOT} -I${SYSROOT}/include -fuse-ld=gold" # Hacky but works
        export CXX="aarch64-unknown-linux-gnu-c++ --sysroot=${SYSROOT}"
        export LD="aarch64-unknown-linux-gnu-ld.gold --sysroot=${SYSROOT}"
        export PKG_CONFIG_PATH="${SYSROOT}/lib/pkgconfig"
        ;;
    "arm")
        export ARCH="arm"
        export GO_ARCH="arm"
        export SYSROOT="${HOME}/x-tools/armv7-centos7-linux-gnueabi/armv7-centos7-linux-gnueabi/sysroot"

        export PATH="${HOME}/x-tools/armv7-centos7-linux-gnueabi/bin:$PATH"
        export CC="armv7-centos7-linux-gnueabi-cc --sysroot=${SYSROOT} -I${SYSROOT}/include -fPIC" # Hacky but works
        export CXX="armv7-centos7-linux-gnueabi-c++ --sysroot=${SYSROOT}"
        export LD="armv7-centos7-linux-gnueabi-ld --sysroot=${SYSROOT}"
        export PKG_CONFIG_PATH="${SYSROOT}/lib/pkgconfig"
        ;;
    "386")
        export ARCH="i686"
        export GO_ARCH="386"
        export SYSROOT="${HOME}/x-tools/i686-centos7-linux-gnu/i686-centos7-linux-gnu/sysroot"

        export PATH="${HOME}/x-tools/i686-centos7-linux-gnu/bin:$PATH"
        export CC="i686-centos7-linux-gnu-cc --sysroot=${SYSROOT} -I${SYSROOT}/include" # Hacky but works
        export CXX="i686-centos7-linux-gnu-c++ --sysroot=${SYSROOT}"
        export LD="i686-centos7-linux-gnu-ld --sysroot=${SYSROOT}"
        export PKG_CONFIG_PATH="${SYSROOT}/lib/pkgconfig"
        ;;
    "amd64")
        export ARCH="amd64"
        export GO_ARCH="amd64"
        export SYSROOT="${HOME}/x-tools/x86_64-centos7-linux-gnu/x86_64-centos7-linux-gnu/sysroot"

        export PATH="${HOME}/x-tools/x86_64-centos7-linux-gnu/bin:$PATH"
        export CC="x86_64-centos7-linux-gnu-cc --sysroot=${SYSROOT} -I${SYSROOT}/include" # Hacky but works
        export CXX="x86_64-centos7-linux-gnu-c++ --sysroot=${SYSROOT}"
        export LD="x86_64-centos7-linux-gnu-ld --sysroot=${SYSROOT}"
        export PKG_CONFIG_PATH="${SYSROOT}/lib/pkgconfig"
        ;;
    *)
        echo "Unknown architecture $1"
        exit 1
        ;;
esac


function build_3rdparty() {
rm -rf 3rdparty/${ARCH}
mkdir -p 3rdparty/${ARCH}

cd 3rdparty/${ARCH}

# Unlock sysroot
chmod -R +w "${SYSROOT}"

# Hack to make Rust linker happy
cd "${SYSROOT}/lib"
rm libgcc_s.so
ln -s libgcc_s.so.1 libgcc_s.so
cd -

# Build and install

#zlib
git clone https://github.com/madler/zlib.git
cd zlib
./configure --prefix="${SYSROOT}"
make -j$(nproc)
make install

cd ..

#libzstd
git clone https://github.com/facebook/zstd.git
cd zstd

make -j$(nproc)
make install PREFIX=${SYSROOT}

cd ..

#libelf
git clone https://github.com/arachsys/libelf.git
cd libelf

# libelf build system is a bit weird, so we need to do this
make -j$(nproc)
make install PREFIX=${SYSROOT}/

cd ..

#libbpf
git clone https://github.com/libbpf/libbpf.git
cd libbpf/src

BUILD_STATIC_ONLY=y EXTRA_CFLAGS=-fPIC DESTDIR=${SYSROOT} V=1 make install install_uapi_headers

cd  ../..

#libtirpc
wget https://zenlayer.dl.sourceforge.net/project/libtirpc/libtirpc/1.3.4/libtirpc-1.3.4.tar.bz2
tar xvf libtirpc-1.3.4.tar.bz2
cd libtirpc-1.3.4

./configure --prefix="${SYSROOT}" --disable-gssapi --host=${ARCH}-centos7-linux-gnu
make -j$(nproc)
make install

cd ..

#libpam
git clone https://github.com/linux-pam/linux-pam.git
cd linux-pam

./autogen.sh
CFLAGS=-fPIC ./configure --prefix="${SYSROOT}" --disable-doc  --disable-examples --includedir="${SYSROOT}/include/security" --host=${ARCH}
make -j$(nproc)
make install

cd ..

touch ready

cd ../..
}

# Build 3rdparty if not already built
if [ ! -f 3rdparty/${ARCH}/ready ]; then
    build_3rdparty
fi

cd ..
# Build teleport
WEBASSETS_SKIP_BUILD=1 GOOS=linux CGO_ENABLED=1 ARCH=${GO_ARCH} go env
WEBASSETS_SKIP_BUILD=1 GOOS=linux CGO_ENABLED=1 ARCH=${GO_ARCH} make
cd e
WEBASSETS_SKIP_BUILD=1 GOOS=linux CGO_ENABLED=1 ARCH=${GO_ARCH} make

# check
readelf -a build/teleport | grep -w -Eo "GLIBC_2\.[0-9]+(\.[0-9]+)?" | sort -u

rm -rf build-${ARCH}
mv -f build/ build-${ARCH}