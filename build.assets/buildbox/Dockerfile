# Build the Teleport buildbox

ARG BASE_IMAGE=ubuntu:22.04
ARG THIRDPARTY_IMAGE

FROM ${THIRDPARTY_IMAGE} AS thirdparty

# Stage 3 is the resulting buildbox, from which we copy what we just build as
# well as install new tools.
FROM ${BASE_IMAGE} AS buildbox

RUN apt-get update && apt-get install -y \
    autoconf \
    automake \
    autopoint \
    bison \
    clang-12 \
    cmake \
    curl \
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

ARG BUILDBOX_UID=99
ARG BUILDBOX_GID=99
RUN groupadd -g $BUILDBOX_GID buildbox
RUN useradd -d /home/buildbox -m -g $BUILDBOX_GID -u $BUILDBOX_UID -s /bin/bash buildbox

RUN install -d -m 0775 -o buildbox -g buildbox /opt/go /opt/rustup /opt/cargo

# Install third party toolchain and libraries
ARG THIRDPARTY_DIR=/opt/thirdparty
COPY --from=thirdparty ${THIRDPARTY_DIR} ${THIRDPARTY_DIR}

USER buildbox

# Install Go
ARG BUILDARCH
ARG GOLANG_VERSION
# Set BUILDARCH if not set when not using buildkit. Only works for arm64 and amd64.
RUN BUILDARCH=${BUILDARCH:-$(uname -m | sed 's/aarch64/arm64/g; s/x86_64/amd64/g')}; \
	curl -fsSL https://storage.googleapis.com/golang/${GOLANG_VERSION}.linux-${BUILDARCH}.tar.gz | \
	tar -C /opt -xz && \
	/opt/go/bin/go version

# Install Rust
ARG RUST_VERSION
ENV RUSTUP_HOME=/opt/rustup
ENV CARGO_HOME=/opt/cargo
RUN curl --proto =https --tlsv1.2 -fsSL https://sh.rustup.rs | \
	sh -s -- -y --profile minimal --default-toolchain ${RUST_VERSION} && \
	${CARGO_HOME}/bin/rustup --version && \
	${CARGO_HOME}/bin/cargo --version && \
	${CARGO_HOME}/bin/rustc --version && \
	${CARGO_HOME}/bin/rustup target add \
		x86_64-unknown-linux-gnu \
		aarch64-unknown-linux-gnu \
		i686-unknown-linux-gnu \
		armv7-unknown-linux-gnueabi \
		wasm32-unknown-unknown

ENV PATH=/opt/go/bin:${CARGO_HOME}/bin:${THIRDPARTY_DIR}/host/bin:${PATH}
ENV THIRDPARTY_DIR=${THIRDPARTY_DIR}

# Set CARGO_HOME, GOPATH and GOCACHE to somewhere writable as the user of the buildbox
# will have a UID/GID different to the buildbox user.
ENV CARGO_HOME=/tmp/cargo
ENV GOPATH=/tmp/go
ENV GOCACHE=/tmp/go/cache

# Add the writable cargo and go bin directories to the path so we will find
# binaries build with `cargo install` and `go install` during a build.
ENV PATH=${CARGO_HOME}/bin:${GOPATH}/bin:${PATH}

# Set a var so the build system can know it's running in this buildbox.
ENV BUILDBOX_MODE=cross
