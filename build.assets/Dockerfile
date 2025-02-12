# syntax=docker/dockerfile:1

###################################################################################
#                                                                                 #
#      DO NOT USE FOR PRODUCTION BUILD OR ANYTHING OTHER THAN CI TESTING!         #
#                                                                                 #
###################################################################################

# This Dockerfile makes the "build box" the container used to:
# * run test and linters in CI
# * building other Docker images
#
# For Teleport releases we're using CentOS 7 box to keep the binaries compatible
# with older Linux distributions (glibc 2.17+).
#
# Check the README to learn how to safely introduce changes to Dockerfiles.

## LIBFIDO2 ###################################################################

# Build libfido2 separately for isolation, speed and flexibility.
FROM buildpack-deps:22.04 AS libfido2

RUN apt-get update && \
    apt-get install -y --no-install-recommends cmake && \
    rm -rf /var/lib/apt/lists/*

# Install libudev-zero.
# libudev-zero replaces systemd's libudev
RUN git clone --depth=1 https://github.com/illiliti/libudev-zero.git -b 1.0.3 && \
    cd libudev-zero && \
    [ "$(git rev-parse HEAD)" = 'ee32ac5f6494047b9ece26e7a5920650cdf46655' ] && \
    make install-static && \
    make clean

# Install libcbor.
RUN git clone --depth=1 https://github.com/PJK/libcbor.git -b v0.11.0 && \
    cd libcbor && \
    [ "$(git rev-parse HEAD)" = '170bee2b82cdb7b2ed25af301f62cb6efdd40ec1' ] && \
    cmake \
        -DCMAKE_BUILD_TYPE=Release \
        -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
        -DWITH_EXAMPLES=OFF . && \
    make && \
    make install && \
    make clean

# Install openssl.
# install_sw install only binaries, skips docs.
RUN git clone --depth=1 https://github.com/openssl/openssl.git -b openssl-3.0.16 && \
    cd openssl && \
    [ "$(git rev-parse HEAD)" = 'fa1e5dfb142bb1c26c3c38a10aafa7a095df52e5' ] && \
    ./config --release -fPIC --libdir=/usr/local/lib && \
    make -j"$(nproc)" && \
    make install_sw

# Install libfido2.
# Depends on libcbor, libcrypto (OpenSSL 3.x), libudev and zlib1g-dev.
# Keep the version below synced with devbox.json
RUN git clone --depth=1 https://github.com/Yubico/libfido2.git -b 1.15.0 && \
    cd libfido2 && \
    [ "$(git rev-parse HEAD)" = 'f87c19c9487c0131531314d9ccb475ea5325794e' ] && \
    CFLAGS=-pthread cmake \
        -DBUILD_EXAMPLES=OFF \
        -DBUILD_MANPAGES=OFF \
        -DBUILD_TOOLS=OFF \
        -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
        -DCMAKE_BUILD_TYPE=Release . && \
    grep 'CRYPTO_VERSION:INTERNAL=3\.0\.' CMakeCache.txt && \
    make && \
    make install && \
    make clean

## LIBBPF #####################################################################

FROM buildpack-deps:22.04 AS libbpf

# Install required dependencies
RUN apt-get update -y --fix-missing && \
    apt-get -q -y upgrade && \
    apt-get install -q -y --no-install-recommends \
        libelf-dev

ARG LIBBPF_VERSION
# BUILD_STATIC_ONLY - builds only static libraries without shared ones
# EXTRA_CFLAGS - additional CFLAGS to pass to the compiler. fPIC is required so the library code can be moved around in memory
# DESTDIR - where to install the library
# V=1 - verbose build
RUN mkdir -p /opt && cd /opt &&  \
    curl -fsSL https://github.com/libbpf/libbpf/archive/refs/tags/v${LIBBPF_VERSION}.tar.gz | tar xz && \
    cd /opt/libbpf-${LIBBPF_VERSION}/src && \
    BUILD_STATIC_ONLY=y EXTRA_CFLAGS=-fPIC DESTDIR=/opt/libbpf V=1 make install install_uapi_headers

## BUILDBOX ###################################################################
#
# Image layers are ordered according to how slow that layer takes to build and
# how frequently it is updated. Slow or infrequently updated dependencies come
# first, fast or frequently updated layers come last.
#
# If you are adding a slow to build and/or complex dependency, consider using a
# multi-stage build for it.
#
# As a rule of thumb, it goes like this:
#
# 1. Slow, language-agnostic dependencies
# 2. Base compilers for main languages
# 3. Fast, language-agnostic dependencies
# 4. Fast, language-dependent dependencies
# 5. Multi-stage layer copies

FROM ubuntu:22.04 AS buildbox

COPY locale.gen /etc/locale.gen
COPY profile /etc/profile
COPY gpg/docker.gpg .
COPY gpg/hashicorp.gpg .
ENV LANGUAGE="en_US.UTF-8" \
    LANG="en_US.UTF-8" \
    LC_ALL="en_US.UTF-8" \
    LC_CTYPE="en_US.UTF-8" \
    DEBIAN_FRONTEND="noninteractive"

# BUILDARCH is automatically set by DOCKER when building the image with Build Kit (MacOS by deafult).
# https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
ARG BUILDARCH

# Install packages.
# Latest git 2.18+ is required for GitHub actions.
# NOTE: gcc-multilib is not available on ARM, so ony amd64 version includes it.
RUN apt-get -y update && \
    apt-get -y install software-properties-common && \
    add-apt-repository -y ppa:git-core/ppa && \
    apt-get update -y --fix-missing && \
    apt-get -q -y upgrade && \
    apt-get install -q -y --no-install-recommends \
        apt-utils \
        build-essential \
        ca-certificates \
        clang \
        clang-format \
        curl \
        `if [ "$BUILDARCH" = "amd64" ] ; then echo gcc-multilib; fi`  \
        git \
        gnupg \
        gzip \
        libc6-dev \
        libelf-dev \
        libpam-dev \
        libpcsclite-dev \
        libsqlite3-0 \
        libssl-dev \
        llvm \
        locales \
        mingw-w64 \
        mingw-w64-x86-64-dev \
        net-tools \
        openssh-client \
        pkg-config \
        python3-pip \
        python3-setuptools \
        python3-wheel \
        # rsync is required for some integration tests
        rsync \
        softhsm2 \
        sudo \
        tree \
        unzip \
        xauth \
        zip \
        zlib1g-dev \
        && \
    install -m 0755 -d /etc/apt/keyrings && \
    gpg --dearmor -o /etc/apt/keyrings/docker.gpg docker.gpg && \
    chmod a+r /etc/apt/keyrings/docker.gpg && \
    echo \
    "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
    "$(. /etc/os-release && echo "$VERSION_CODENAME")" stable" | \
    tee /etc/apt/sources.list.d/docker.list > /dev/null && \
    apt-get update && \
    apt-get install -y docker-ce-cli && \
    gpg --dearmor -o /usr/share/keyrings/hashicorp.gpg hashicorp.gpg && \
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/hashicorp.gpg] https://apt.releases.hashicorp.com $(lsb_release -cs) main" | \
    tee /etc/apt/sources.list.d/hashicorp.list > /dev/null && \
    apt-get update && apt-get install terraform -y --no-install-recommends && \
    # Manually install the wasm-opt binary from the binaryen package on ARM64.
    if [ "$BUILDARCH" = "arm64" ]; then apt-get install -y binaryen; fi && \
    pip3 --no-cache-dir install yamllint && \
    dpkg-reconfigure locales && \
    apt-get -y clean && \
    rm -rf /var/lib/apt/lists/*

# Install osslsigncode for Windows Code Signing. The format of our
# Code Signing Certificate needs us to use osslsigncode >= 2.6, which
# allows the use of legacy OpenSSL algorithms. This is not yet provided
# by Ubuntu, so we will have to fetch it ourselves.
RUN --mount=type=bind,source=download-hashes/osslsigncode.sha256,target=/context/osslsigncode.sha256 \
    curl -L https://github.com/mtrojnar/osslsigncode/releases/download/2.6/osslsigncode-2.6-ubuntu-22.04.zip -o osslsigncode.zip \
    && sha256sum --strict -c /context/osslsigncode.sha256 \
    && unzip -d /tmp/osslsigncode osslsigncode.zip \
    && install -m 0755 /tmp/osslsigncode/bin/osslsigncode /usr/bin \
    && rm -rf /tmp/osslsigncode \
    && rm osslsigncode.zip

# Add the CI user.
ARG UID
ARG GID
RUN groupadd ci --gid=$GID -o && \
    useradd ci --uid=$UID --gid=$GID --create-home --shell=/bin/sh && \
    mkdir -p -m0700 /var/lib/teleport && chown -R ci /var/lib/teleport

# Install Rust.
ARG RUST_VERSION
ENV RUSTUP_HOME=/usr/local/rustup \
     CARGO_HOME=/usr/local/cargo \
     PATH=/usr/local/cargo/bin:$PATH \
     RUST_VERSION=$RUST_VERSION
RUN mkdir -p $RUSTUP_HOME && chmod a+w $RUSTUP_HOME && \
    mkdir -p $CARGO_HOME/registry && chmod -R a+w $CARGO_HOME
# Install Rust using the ci user, as that is the user that
# will run builds using the Rust toolchains we install here.
# Cross-compilation targets are only installed on amd64, as
# this image doesn't contain gcc-multilib.
USER ci
RUN curl --proto '=https' --tlsv1.2 -fsSL https://sh.rustup.rs | sh -s -- -y --profile minimal --default-toolchain $RUST_VERSION && \
    rustup --version && \
    cargo --version && \
    rustc --version && \
    rustup component add rustfmt clippy && \
    rustup target add wasm32-unknown-unknown && \
    if [ "$BUILDARCH" = "amd64" ]; then rustup target add aarch64-unknown-linux-gnu i686-unknown-linux-gnu; fi

ARG WASM_PACK_VERSION
# Install wasm-pack for targeting WebAssembly from Rust.
RUN cargo install wasm-pack --locked --version ${WASM_PACK_VERSION}

# Switch back to root for the remaining instructions and keep it as the default
# user.
USER root

# Install Node.js.
ARG NODE_VERSION
ENV NODE_PATH="/usr/local/lib/nodejs-linux"
ENV PATH="$PATH:${NODE_PATH}/bin"
RUN export NODE_ARCH=$(if [ "$BUILDARCH" = "amd64" ]; then echo "x64"; else echo "arm64"; fi) && \
    export NODE_URL="https://nodejs.org/dist/v${NODE_VERSION}/node-v${NODE_VERSION}-linux-${NODE_ARCH}.tar.xz" && \
    mkdir -p ${NODE_PATH} && \
    curl -o /tmp/nodejs.tar.xz -fsSL ${NODE_URL} && \
    tar -xJf /tmp/nodejs.tar.xz -C /usr/local/lib/nodejs-linux --strip-components=1
RUN npm install -g corepack@0.31.0 && corepack enable yarn pnpm

# Install Go.
ARG GOLANG_VERSION
RUN mkdir -p /opt && cd /opt && curl -fsSL https://storage.googleapis.com/golang/$GOLANG_VERSION.linux-${BUILDARCH}.tar.gz | tar xz && \
    mkdir -p /go/src/github.com/gravitational/teleport && \
    chmod a+w /go && \
    chmod a+w /var/lib && \
    chmod a-w /
ENV GOPATH="/go" \
    GOROOT="/opt/go" \
    PATH="$PATH:/opt/go/bin:/go/bin:/go/src/github.com/gravitational/teleport/build"

# Install PAM module and policies for testing.
COPY pam/ /opt/pam_teleport/
RUN make -C /opt/pam_teleport install
ENV SOFTHSM2_PATH "/usr/lib/softhsm/libsofthsm2.so"

# Install bats.
RUN git clone --depth=1 https://github.com/bats-core/bats-core.git -b v1.2.1 && \
    cd bats-core && \
    [ "$(git rev-parse HEAD)" = 'dcaec03e32e0b152f8ef9cf14b75296cf5caeaff' ] && \
    ./install.sh /usr/local && cd .. && \
    rm -r bats-core

# Install shellcheck.
RUN scversion='v0.10.0' && \
    curl -fsSL "https://github.com/koalaman/shellcheck/releases/download/$scversion/shellcheck-$scversion.linux.$(if [ "$BUILDARCH" = "amd64" ]; then echo "x86_64"; else echo "aarch64"; fi).tar.xz" | \
        tar -xJv && \
    cp "shellcheck-$scversion/shellcheck" /usr/local/bin/ && \
    shellcheck --version

# Install helm.
# Keep the version below synced with devbox.json
RUN mkdir -p helm-tarball && \
    curl -fsSL https://get.helm.sh/helm-v3.12.2-$(go env GOOS)-$(go env GOARCH).tar.gz | tar -C helm-tarball -xz && \
    cp helm-tarball/$(go env GOOS)-$(go env GOARCH)/helm /bin/ && \
    rm -r helm-tarball*
# TODO(hugoShaka): remove this backward compatible hack with teleportv13 buildbox
RUN helm plugin install https://github.com/quintush/helm-unittest --version 0.2.11 && \
    mkdir -p /home/ci/.local/share/helm && \
    cp -r /root/.local/share/helm/plugins /home/ci/.local/share/helm/plugins-new && \
    chown -R ci /home/ci/.local/share/helm && \
    helm plugin uninstall unittest && \
    HELM_PLUGINS=/home/ci/.local/share/helm/plugins-new helm plugin list
RUN helm plugin install https://github.com/vbehar/helm3-unittest && \
    mkdir -p /home/ci/.local/share/helm && \
    cp -r /root/.local/share/helm/plugins /home/ci/.local/share/helm && \
    chown -R ci /home/ci/.local/share/helm && \
    HELM_PLUGINS=/home/ci/.local/share/helm/plugins helm plugin list

# Install JS gRPC tools.
ARG NODE_GRPC_TOOLS_VERSION # eg, "1.12.4"
ARG NODE_PROTOC_TS_VERSION  # eg, "5.0.1"
RUN npm install --global "grpc-tools@$NODE_GRPC_TOOLS_VERSION" "grpc_tools_node_protoc_ts@$NODE_PROTOC_TS_VERSION"

# Install protoc.
ARG PROTOC_VERSION # eg, "26.1"
RUN VERSION="$PROTOC_VERSION" && \
  PB_REL='https://github.com/protocolbuffers/protobuf/releases' && \
  PB_FILE="$(mktemp protoc-XXXXXX.zip)" && \
  curl -fsSL -o "$PB_FILE" "$PB_REL/download/v$VERSION/protoc-$VERSION-linux-$(if [ "$BUILDARCH" = "amd64" ]; then echo "x86_64"; else echo "aarch_64"; fi).zip"  && \
  unzip "$PB_FILE" -d /usr/local && \
  rm -f "$PB_FILE"

# Install protoc-gen-gogofast.
ARG GOGO_PROTO_TAG # eg, "v1.3.2"
RUN go install "github.com/gogo/protobuf/protoc-gen-gogofast@$GOGO_PROTO_TAG"

# Install addlicense.
RUN go install github.com/google/addlicense@v1.0.0

# Install GCI.
RUN go install github.com/daixiang0/gci@v0.13.5

# Install gotestsum.
RUN go install gotest.tools/gotestsum@v1.10.1

# Install golangci-lint.
ARG GOLANGCI_LINT_VERSION # eg, v1.56.1
RUN VERSION="$GOLANGCI_LINT_VERSION"; \
    curl -fsSL "https://raw.githubusercontent.com/golangci/golangci-lint/$VERSION/install.sh" | \
    sh -s -- -b "$(go env GOPATH)/bin" "$VERSION"

# Install Buf.
ARG BUF_VERSION # eg, "v1.26.1"
RUN VERSION="$BUF_VERSION"; \
    go install "github.com/bufbuild/buf/cmd/buf@$VERSION"

# Copy BPF libraries.
ARG LIBBPF_VERSION
COPY --from=libbpf /opt/libbpf/usr /usr/libbpf-${LIBBPF_VERSION}

# Copy libfido2 libraries.
# Do this near the end to take better advantage of the multi-stage build.
COPY --from=libfido2 /usr/local/include/ /usr/local/include/
COPY --from=libfido2 /usr/local/lib/engines-3/ /usr/local/lib/engines-3/
COPY --from=libfido2 /usr/local/lib/ossl-modules/ /usr/local/lib/ossl-modules/
COPY --from=libfido2 /usr/local/lib/pkgconfig/ /usr/local/lib/pkgconfig/
COPY --from=libfido2 \
    /usr/local/lib/libcbor.a \
    /usr/local/lib/libcrypto.a \
    /usr/local/lib/libcrypto.so.3 \
    /usr/local/lib/libfido2.a \
    /usr/local/lib/libfido2.so.1.15.0 \
    /usr/local/lib/libssl.a \
    /usr/local/lib/libssl.so.3 \
    /usr/local/lib/libudev.a \
    /usr/local/lib/
RUN cd /usr/local/lib && \
    ln -s libcrypto.so.3 libcrypto.so && \
    ln -s libfido2.so.1.15.0 libfido2.so.1 && \
    ln -s libfido2.so.1 libfido2.so && \
    ln -s libssl.so.3 libssl.so && \
    ldconfig
COPY pkgconfig/buildbox/ /

VOLUME ["/go/src/github.com/gravitational/teleport"]
EXPOSE 6600 2379 2380
