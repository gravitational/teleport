# syntax=docker/dockerfile:1
#
# Build the Teleport buildbox with the pre-built crosstool-NG cross compilers,
# pre-built third-party C libraries for the four supported architectures, the
# Go toolchain and the Rust toolchain.
#
# The pre-built crosstool-NG cross compilers and third-party C libraries come
# from a image specified by the THIRDPARTY_IMAGE arg, which is built by the
# Dockerfile-thirdparty dockerfile.
#
# All the compilers/toolchains are owned by the buildbox:buildbox (99:99) user.
# They are not intended to be modifiable when the buildbox runs. By default the
# image runs as user teleport:teleport (1000:1000) and can write to
# /home/teleport. /home/teleport is also world-writable so if the UID/GID
# 1000/1000 cannot be used for some reason, the user can still check out
# repositories into that directory.
#
# Alternatively, map an external repository directory as a volume on to
# /home/teleport and set the uid/gid to the owner of that directory.
#
# The default GOPATH is /tmp/go, with the build cache (GOCACHE) in
# /tmp/go/build. CARGO_HOME is /tmp/rust. This allows Go and Rust to run and
# have a place to cache builds and install binaries.

ARG BASE_IMAGE=ubuntu:22.04
ARG THIRDPARTY_IMAGE

# ----------------------------------------------------------------------------
# Define a simple base image for installing various compilers which are then
# copied into the final image. This helps with caching as the download+install
# does not depend on previous layers.

FROM ${BASE_IMAGE} AS base

# Install curl as it is needed by the Go and Rust stages to download the compilers.
RUN apt-get update \
    && apt-get install -y curl \
    && rm -rf /var/lib/apt/lists/*

ARG BUILDBOX_UID=99
ARG BUILDBOX_GID=99
RUN groupadd -g $BUILDBOX_GID buildbox
RUN useradd -d /home/buildbox -m -g $BUILDBOX_GID -u $BUILDBOX_UID -s /bin/bash buildbox

ARG TELEPORT_UID=1000
ARG TELEPORT_GID=1000
RUN groupadd -g $TELEPORT_GID teleport
RUN useradd -d /home/teleport -m -g $TELEPORT_GID -u $TELEPORT_UID -s /bin/bash teleport
RUN chmod 777 /home/teleport

# ----------------------------------------------------------------------------
# Reference the thirdparty image for copying from later.

FROM ${THIRDPARTY_IMAGE} AS thirdparty

# ----------------------------------------------------------------------------
# Install Go
#
# Go is installed into the base image and copied across to the final image in
# the last stage. This make the downloading and installation of the Go
# toolchain dependent on nothing but the base.

FROM base AS go

RUN install -d -m 0775 -o buildbox -g buildbox /opt/go
USER buildbox

ARG BUILDARCH
ARG GOLANG_VERSION
# Set BUILDARCH if not set when not using buildkit. Only works for arm64 and amd64.
RUN BUILDARCH=${BUILDARCH:-$(uname -m | sed 's/aarch64/arm64/g; s/x86_64/amd64/g')}; \
	curl -fsSL https://storage.googleapis.com/golang/${GOLANG_VERSION}.linux-${BUILDARCH}.tar.gz | \
	tar -C /opt -xz && \
	/opt/go/bin/go version

# ----------------------------------------------------------------------------
# Install Rust
#
# Rust is installed into the base image and copied across to the final image in
# the last stage. This make the downloading and installation of the Rust
# toolchain dependent on nothing but the base.

FROM base AS rust

RUN install -d -m 0775 -o buildbox -g buildbox /opt/rust
USER buildbox

ARG RUST_VERSION
ENV RUSTUP_HOME=/opt/rust
ENV CARGO_HOME=/opt/rust
RUN curl --proto =https --tlsv1.2 -fsSL https://sh.rustup.rs | \
	sh -s -- -y --profile minimal --default-toolchain ${RUST_VERSION} && \
	${CARGO_HOME}/bin/rustup --version && \
	${CARGO_HOME}/bin/cargo --version && \
	${CARGO_HOME}/bin/rustc --version && \
	${CARGO_HOME}/bin/rustup target add \
		x86_64-unknown-linux-gnu \
		aarch64-unknown-linux-gnu \
		i686-unknown-linux-gnu \
		arm-unknown-linux-gnueabihf \
		wasm32-unknown-unknown

# ----------------------------------------------------------------------------
# Clang 12.0.0 for FIPS builds of boring-rs

FROM base AS clang

# libtinfo5 required to run clang to test it works.
# xz-utils for decompressing the clang tarball.
RUN apt-get update \
    && apt-get install -y libtinfo5 xz-utils \
    && rm -rf /var/lib/apt/lists/*

RUN install -d -m 0775 -o buildbox -g buildbox /opt/clang
USER buildbox

# TODO(camscale): Verify signature of download.
RUN \
	case "$(uname -m)" in \
		aarch64|arm64) SUFFIX='aarch64-linux-gnu.tar.xz' ;; \
		x86_64|amd64) SUFFIX='x86_64-linux-gnu-ubuntu-20.04.tar.xz' ;; \
		*) echo "Unsupported architecture for clang: $(uname -m)" >&2; exit 1 ;; \
	esac; \
	curl -fsSL "https://github.com/llvm/llvm-project/releases/download/llvmorg-12.0.0/clang+llvm-12.0.0-${SUFFIX}" | \
	tar -C /opt/clang -xJ --strip-components=1 && \
	/opt/clang/bin/clang --version

# ----------------------------------------------------------------------------
# buildbox image
#
# Build the final buildbox image by installing required packages and copying
# the toolchains from the previous stages/images.

FROM base AS buildbox

RUN apt-get update && apt-get install -y \
    autoconf \
    automake \
    autopoint \
    bison \
    cmake \
    flex \
    gettext \
    git \
    libtinfo5 \
    libtool \
    make \
    ninja-build \
    pkg-config \
    sed \
    w3m \
    wget \
    xsltproc \
    xz-utils \
    && rm -rf /var/lib/apt/lists/*

RUN install -d -m 1777 -o teleport -g teleport /tmp/build

# The boring-rs build wants llvm-{ar,ranlib}-12 in /usr/bin. The
# clang install has /opt/clang/bin/llvm-{ar,ranlib}. Create /usr/bin
# symlinks while we're still root
RUN \
	ln -nsf /opt/clang/bin/llvm-ar /usr/bin/llvm-ar-12 && \
	ln -nsf /opt/clang/bin/llvm-ranlib /usr/bin/llvm-ranlib-12

USER buildbox

# Copy compilers from other images
ARG THIRDPARTY_DIR=/opt/thirdparty
COPY --from=thirdparty ${THIRDPARTY_DIR} ${THIRDPARTY_DIR}
COPY --from=rust /opt/rust /opt/rust
COPY --from=go /opt/go /opt/go
COPY --from=clang /opt/clang /opt/clang

# The boring-rs build uses cmake which wants clang++ to be called clang++-12.
RUN ln -nsf clang /opt/clang/bin/clang++-12

# We need a clang front-end script to set some command line args to properly
# find/use the appropriate cross-compiling gcc toolchain, to build boring-rs
# in FIPS mode.
COPY clang-12.sh /opt/clang/bin
RUN \
	cd /opt/clang/bin && \
	mv clang-12 clang-12.bin && \
	ln -s clang-12.sh clang-12

# Set RUSTUP_HOME so cargo does not warn/error about not finding it at ~/.cargo
ENV RUSTUP_HOME=/opt/rust

ENV PATH=/opt/go/bin:/opt/rust/bin:${THIRDPARTY_DIR}/host/bin:${PATH}

# Ensure THIRDPARTY_DIR gets propagated to the makefiles if the provided arg
# is not the default value.
ENV THIRDPARTY_DIR=${THIRDPARTY_DIR}

# Set up env vars for rust to cross-compile binaries. I needs a linker for the
# appropriate architecture, which is invoked via `cc`. These compilers are all
# on the PATH.
ENV CARGO_TARGET_X86_64_UNKNOWN_LINUX_GNU_LINKER=x86_64-unknown-linux-gnu-gcc
ENV CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER=aarch64-unknown-linux-gnu-gcc
ENV CARGO_TARGET_I686_UNKNOWN_LINUX_GNU_LINKER=i686-unknown-linux-gnu-gcc
ENV CARGO_TARGET_ARM_UNKNOWN_LINUX_GNUEABIHF_LINKER=arm-unknown-linux-gnueabihf-gcc

# Set CARGO_HOME, GOPATH and GOCACHE to somewhere writable as the user of the
# buildbox will have a UID/GID different to the buildbox user. Also create
# /home/teleport as a world-writable directory so that can be used as a
# workspace when cloning a git repo directly in the buildbox as opposed to
# mapping a volume from outside.
# The /tmp/build directory can be a volume so the build/package cache can be
# carried across builds.
ENV CARGO_HOME=/tmp/build/rust
ENV GOPATH=/tmp/build/go
ENV GOCACHE=/tmp/build/go/build

# Add the writable cargo and go bin directories to the path so we will find
# binaries build with `cargo install` and `go install` during a build.
ENV PATH=${CARGO_HOME}/bin:${GOPATH}/bin:/opt/clang/bin:${PATH}

# Set a var so the build system can know it's running in this buildbox.
ENV BUILDBOX_MODE=cross

USER teleport:teleport
WORKDIR /home/teleport
