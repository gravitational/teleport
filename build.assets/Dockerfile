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
FROM buildpack-deps:20.04 AS libfido2

RUN apt-get update && \
    apt-get install -y --no-install-recommends cmake && \
    rm -rf /var/lib/apt/lists/*

# Install libudev-zero.
# libudev-zero replaces systemd's libudev
RUN git clone --depth=1 https://github.com/illiliti/libudev-zero.git -b 1.0.1 && \
    cd libudev-zero && \
    [ "$(git rev-parse HEAD)" = '4154cf252c17297f98a8ca33693ead003b4509da' ] && \
    make install-static && \
    make clean

# Install libcbor.
RUN git clone --depth=1 https://github.com/PJK/libcbor.git -b v0.10.2 && \
    cd libcbor && \
    [ "$(git rev-parse HEAD)" = 'efa6c0886bae46bdaef9b679f61f4b9d8bc296ae' ] && \
    cmake \
        -DCMAKE_BUILD_TYPE=Release \
        -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
        -DWITH_EXAMPLES=OFF . && \
    make && \
    make install && \
    make clean

# Install openssl.
# install_sw install only binaries, skips docs.
RUN git clone --depth=1 https://github.com/openssl/openssl.git -b openssl-3.0.9 && \
    cd openssl && \
    [ "$(git rev-parse HEAD)" = 'de90e54bbe82e5be4fb9608b6f5c308bb837d355' ] && \
    ./config --release --libdir=/usr/local/lib && \
    make -j"$(nproc)" && \
    make install_sw

# Install libfido2.
# Depends on libcbor, libcrypto (OpenSSL 3.x), libudev and zlib1g-dev.
RUN git clone --depth=1 https://github.com/Yubico/libfido2.git -b 1.13.0 && \
    cd libfido2 && \
    [ "$(git rev-parse HEAD)" = '486a8f8667e42f55cee2bba301b41433cacec830' ] && \
    CFLAGS=-pthread cmake \
        -DBUILD_EXAMPLES=OFF \
        -DBUILD_MANPAGES=OFF \
        -DBUILD_TOOLS=OFF \
        -DCMAKE_BUILD_TYPE=Release . && \
    grep 'CRYPTO_VERSION:INTERNAL=3\.0\.' CMakeCache.txt && \
    make && \
    make install && \
    make clean

## LIBBPF #####################################################################

FROM buildpack-deps:20.04 AS libbpf

# Install required dependencies
RUN apt-get update -y --fix-missing && \
    apt-get -q -y upgrade && \
    apt-get install -q -y --no-install-recommends \
        libelf-dev

ARG LIBBPF_VERSION
RUN mkdir -p /opt && cd /opt &&  \
    curl -fsSL https://github.com/libbpf/libbpf/archive/refs/tags/v${LIBBPF_VERSION}.tar.gz | tar xz && \
    cd /opt/libbpf-${LIBBPF_VERSION}/src && \
    make &&  \
    BUILD_STATIC_ONLY=y DESTDIR=/opt/libbpf make install

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

FROM ubuntu:20.04 AS buildbox

COPY locale.gen /etc/locale.gen
COPY profile /etc/profile
COPY gpg/docker.gpg .
ENV LANGUAGE="en_US.UTF-8" \
    LANG="en_US.UTF-8" \
    LC_ALL="en_US.UTF-8" \
    LC_CTYPE="en_US.UTF-8" \
    DEBIAN_FRONTEND="noninteractive"

# BUILDARCH is automatically set by DOCKER when building the image with Build Kit (MacOS by deafult).
# https://docs.docker.com/engine/reference/builder/#automatic-platform-args-in-the-global-scope
ARG BUILDARCH

# Install packages.
# Java JRE is required by gcloud firestore emulator.
# Latest git 2.18+ is required for GitHub actions
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
        default-jre \
        `if [ "$BUILDARCH" = "amd64" ] ; then echo gcc-multilib; fi`  \
        git \
        gnupg \
        gzip \
        libc6-dev \
        libelf-dev \
        libpam-dev \
        libsqlite3-0 \
        libssl-dev \
        llvm \
        locales \
        mingw-w64 \
        mingw-w64-x86-64-dev \
        net-tools \
        openssh-client \
        osslsigncode \
        python3-pip \
        python3-setuptools \
        python3-wheel \
        pkg-config \
        # Used during tag builds to build the RPM package of Connect.
        rpm \
        softhsm2 \
        sudo \
        tree \
        unzip \
        zip \
        zlib1g-dev \
        xauth \
        libpcsclite-dev \
        && \
    install -m 0755 -d /etc/apt/keyrings && \
    gpg --dearmor -o /etc/apt/keyrings/docker.gpg docker.gpg && \
    chmod a+r /etc/apt/keyrings/docker.gpg && \
    echo \
    "deb [arch="$(dpkg --print-architecture)" signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
    "$(. /etc/os-release && echo "$VERSION_CODENAME")" stable" | \
    tee /etc/apt/sources.list.d/docker.list > /dev/null && \
    apt-get update && \
    apt-get install -y docker-ce-cli && \
    pip3 --no-cache-dir install yamllint && \
    dpkg-reconfigure locales && \
    apt-get -y clean && \
    rm -rf /var/lib/apt/lists/*

# Install gcloud SDK and Firestore emulator.
ENV PATH="$PATH:/opt/google-cloud-sdk/bin"
RUN (curl -fsSL https://sdk.cloud.google.com | bash -s -- --install-dir=/opt --disable-prompts) && \
    gcloud components install cloud-firestore-emulator beta && \
    rm -rf /opt/google-cloud-sdk/.install/.backup

# Install etcd.
RUN curl -fsSL https://github.com/coreos/etcd/releases/download/v3.3.9/etcd-v3.3.9-linux-${BUILDARCH}.tar.gz | tar -xz && \
    cp etcd-v3.3.9-linux-${BUILDARCH}/etcd* /bin/ && \
    rm -rf etcd-v3.3.9-linux-${BUILDARCH}

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
    if [ "$BUILDARCH" = "amd64" ]; then rustup target add aarch64-unknown-linux-gnu; fi
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
RUN corepack enable yarn

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
RUN curl -fsSL https://github.com/bats-core/bats-core/archive/v1.2.1.tar.gz | tar -xz && \
    cd bats-core-1.2.1 && ./install.sh /usr/local && cd .. && \
    rm -r bats-core-1.2.1

# Install shellcheck.
RUN scversion='v0.9.0' && \
    curl -fsSL "https://github.com/koalaman/shellcheck/releases/download/$scversion/shellcheck-$scversion.linux.$(if [ "$BUILDARCH" = "amd64" ]; then echo "x86_64"; else echo "aarch64"; fi).tar.xz" | \
        tar -xJv && \
    cp "shellcheck-$scversion/shellcheck" /usr/local/bin/ && \
    shellcheck --version

# Install helm.
RUN mkdir -p helm-tarball && \
    curl -fsSL https://get.helm.sh/helm-v3.5.2-$(go env GOOS)-$(go env GOARCH).tar.gz | tar -C helm-tarball -xz && \
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
ARG PROTOC_VER # eg, "3.20.2"
RUN VERSION="$PROTOC_VER" && \
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
RUN go install github.com/daixiang0/gci@v0.9.1

# Install golangci-lint.
RUN TAG='v1.53.3' && \
    curl -fsSL "https://raw.githubusercontent.com/golangci/golangci-lint/$TAG/install.sh" | \
    sh -s -- -b "$(go env GOPATH)/bin" "$TAG"

# Install Buf.
ARG BUF_VERSION # eg, "1.19.0"
RUN BIN='/usr/local/bin' && \
  VERSION="$BUF_VERSION" && \
  curl -fsSL "https://github.com/bufbuild/buf/releases/download/v$VERSION/buf-$(uname -s)-$(uname -m)" -o "${BIN}/buf" && \
  chmod +x "$BIN/buf"

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
    /usr/local/lib/libfido2.so.1.13.0 \
    /usr/local/lib/libssl.a \
    /usr/local/lib/libssl.so.3 \
    /usr/local/lib/libudev.a \
    /usr/local/lib/
RUN cd /usr/local/lib && \
    ln -s libcrypto.so.3 libcrypto.so && \
    ln -s libfido2.so.1.13.0 libfido2.so.1 && \
    ln -s libfido2.so.1 libfido2.so && \
    ln -s libssl.so.3 libssl.so && \
    ldconfig
COPY pkgconfig/buildbox/ /

VOLUME ["/go/src/github.com/gravitational/teleport"]
EXPOSE 6600 2379 2380
