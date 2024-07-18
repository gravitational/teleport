# Build the Teleport buildbox

ARG BASE_IMAGE=ubuntu:22.04
ARG THIRDPARTY_IMAGE

# ----------------------------------------------------------------------------
# Define a simple base image for installing various compilers which are then
# copied into the final image. This helps with caching as the download+install
# does not depend on previous layers.

FROM ${BASE_IMAGE} AS base

ARG BUILDBOX_UID=99
ARG BUILDBOX_GID=99
RUN groupadd -g $BUILDBOX_GID buildbox
RUN useradd -d /home/buildbox -m -g $BUILDBOX_GID -u $BUILDBOX_UID -s /bin/bash buildbox

# Install curl as it is needed by the Go and Rust stages to download the compilers.
RUN apt-get update \
    && apt-get install -y curl \
    && rm -rf /var/lib/apt/lists/*

# ----------------------------------------------------------------------------
# Reference the thirdparty image for copying from later.

FROM ${THIRDPARTY_IMAGE} AS thirdparty

# ----------------------------------------------------------------------------
# Install Go

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
# buildbox image

FROM base AS buildbox

RUN apt-get update && apt-get install -y \
    autoconf \
    automake \
    autopoint \
    bison \
    clang-12 \
    cmake \
    flex \
    gettext \
    git \
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

USER buildbox

# Copy compilers from other images
ARG THIRDPARTY_DIR=/opt/thirdparty
COPY --from=thirdparty ${THIRDPARTY_DIR} ${THIRDPARTY_DIR}
COPY --from=rust /opt/rust /opt/rust
COPY --from=go /opt/go /opt/go

ENV PATH=/opt/go/bin:/opt/rust/bin:${THIRDPARTY_DIR}/host/bin:${PATH}
ENV THIRDPARTY_DIR=${THIRDPARTY_DIR}

# Set CARGO_HOME, GOPATH and GOCACHE to somewhere writable as the user of the buildbox
# will have a UID/GID different to the buildbox user.
ENV RUSTUP_HOME=/opt/rust
ENV CARGO_HOME=/tmp/rust
ENV GOPATH=/tmp/go
ENV GOCACHE=/tmp/go/cache

# Set up env vars for rust to cross-compile binaries. I needs a linker for the
# appropriate architecture, which is invoked via `cc`. These compilers are all
# on the PATH.
ENV CARGO_TARGET_X86_64_UNKNOWN_LINUX_GNU_LINKER=x86_64-unknown-linux-gnu-gcc
ENV CARGO_TARGET_AARCH64_UNKNOWN_LINUX_GNU_LINKER=aarch64-unknown-linux-gnu-gcc
ENV CARGO_TARGET_I686_UNKNOWN_LINUX_GNU_LINKER=i686-unknown-linux-gnu-gcc
ENV CARGO_TARGET_ARM_UNKNOWN_LINUX_GNUEABIHF_LINKER=arm-unknown-linux-gnueabihf-gcc

# Add the writable cargo and go bin directories to the path so we will find
# binaries build with `cargo install` and `go install` during a build.
ENV PATH=${CARGO_HOME}/bin:${GOPATH}/bin:${PATH}

# Set a var so the build system can know it's running in this buildbox.
ENV BUILDBOX_MODE=cross
