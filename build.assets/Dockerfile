# This Dockerfile makes the "build box": the container used to build official
# releases of Teleport and its documentation.

# Use Ubuntu 18.04 as base to get an older glibc version.
# Using a newer base image will build against a newer glibc, which creates a
# runtime requirement for the host to have newer glibc too. For example,
# teleport built on any newer Ubuntu version will not run on Centos 7 because
# of this.
FROM ubuntu:18.04

COPY locale.gen /etc/locale.gen
COPY profile /etc/profile

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
# NOTE: gcc-multilib is not available on ARM, so ony amd64 version includes it.
RUN apt-get update -y --fix-missing && \
    apt-get -q -y upgrade && \
    apt-get install -q -y --no-install-recommends \
        apt-utils \
        clang-10 \
        clang-format-10 \
        ca-certificates \
        curl \
        default-jre \
        gcc \
        `if [ "$BUILDARCH" = "amd64" ] ; then echo gcc-multilib; fi`  \
        git \
        gnupg \
        gzip \
        libc6-dev \
        libelf-dev \
        libpam-dev \
        libsqlite3-0 \
        llvm-10 \
        locales \
        make \
        mingw-w64 \
        mingw-w64-x86-64-dev \
        net-tools \
        openssh-client \
        osslsigncode \
        python3-pip \
        python3-setuptools \
        python3-wheel \
        pkg-config \
        shellcheck \
        softhsm2 \
        tar \
        tree \
        unzip \
        zip \
        zlib1g-dev \
        && \
    pip3 --no-cache-dir install yamllint && \
    dpkg-reconfigure locales && \
    apt-get -y clean && \
    rm -rf /var/lib/apt/lists/*

# Install gcloud SDK and Firestore emulator.
RUN curl -sSL https://sdk.cloud.google.com | bash
ENV PATH="$PATH:/root/google-cloud-sdk/bin"
# beta component is required by firestore emulator: https://cloud.google.com/sdk/gcloud/reference/beta/emulators/firestore/start
RUN gcloud components install cloud-firestore-emulator beta

ARG UID
ARG GID
RUN (groupadd ci --gid=$GID -o && useradd ci --uid=$UID --gid=$GID --create-home --shell=/bin/sh && \
     mkdir -p -m0700 /var/lib/teleport && chown -R ci /var/lib/teleport)

# Install etcd.
RUN (curl -L https://github.com/coreos/etcd/releases/download/v3.3.9/etcd-v3.3.9-linux-${BUILDARCH}.tar.gz | tar -xz && \
     cp etcd-v3.3.9-linux-${BUILDARCH}/etcd* /bin/)

# Install Go.
ARG RUNTIME
RUN mkdir -p /opt && cd /opt && curl https://storage.googleapis.com/golang/$RUNTIME.linux-${BUILDARCH}.tar.gz | tar xz && \
    mkdir -p /go/src/github.com/gravitational/teleport && \
    chmod a+w /go && \
    chmod a+w /var/lib && \
    chmod a-w /

# Install libbpf
ARG LIBBPF_VERSION
RUN mkdir -p /opt && cd /opt && curl -L https://github.com/gravitational/libbpf/archive/refs/tags/v${LIBBPF_VERSION}.tar.gz | tar xz && \
    cd /opt/libbpf-${LIBBPF_VERSION}/src && \
    make && \
    make install

ENV GOPATH="/go" \
    GOROOT="/opt/go" \
    PATH="$PATH:/opt/go/bin:/go/bin:/go/src/github.com/gravitational/teleport/build"

# Install addlicense
RUN go install github.com/google/addlicense@v1.0.0

# Install meta-linter.
RUN (curl -L https://github.com/golangci/golangci-lint/releases/download/v1.38.0/golangci-lint-1.38.0-$(go env GOOS)-$(go env GOARCH).tar.gz | tar -xz && \
     cp golangci-lint-1.38.0-$(go env GOOS)-$(go env GOARCH)/golangci-lint /bin/ && \
     rm -r golangci-lint*)

# Install helm.
RUN (mkdir -p helm-tarball && curl -L https://get.helm.sh/helm-v3.5.2-$(go env GOOS)-$(go env GOARCH).tar.gz | tar -C helm-tarball -xz && \
     cp helm-tarball/$(go env GOOS)-$(go env GOARCH)/helm /bin/ && \
     rm -r helm-tarball*)

# Install bats.
RUN (curl -L https://github.com/bats-core/bats-core/archive/v1.2.1.tar.gz | tar -xz && \
     cd bats-core-1.2.1 && ./install.sh /usr/local && cd .. && \
     rm -r bats-core-1.2.1)

# Install protobuf and grpc build tools.
ARG PROTOC_VER
ARG GOGO_PROTO_TAG
ENV GOGOPROTO_ROOT ${GOPATH}/src/github.com/gogo/protobuf

RUN (export PROTOC_TARBALL=protoc-${PROTOC_VER}-linux-$(if [ "$BUILDARCH" = "amd64" ]; then echo "x86_64"; else echo "aarch_64"; fi).zip && \
     curl -L -o /tmp/${PROTOC_TARBALL} https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VER}/${PROTOC_TARBALL} && \
     cd /tmp && unzip /tmp/${PROTOC_TARBALL} -d /usr/local && \
     chmod -R a+r /usr/local/include/google/protobuf && \
     rm /tmp/${PROTOC_TARBALL})
RUN (git clone https://github.com/gogo/protobuf.git ${GOPATH}/src/github.com/gogo/protobuf && \
     cd ${GOPATH}/src/github.com/gogo/protobuf && \
     git reset --hard ${GOGO_PROTO_TAG} && \
     make install)

ENV PROTO_INCLUDE "/usr/local/include":"/go/src":"/go/src/github.com/gogo/protobuf/protobuf":"${GOGOPROTO_ROOT}":"${GOGOPROTO_ROOT}/protobuf"

# Install PAM module and policies for testing.
COPY pam/ /opt/pam_teleport/
RUN make -C /opt/pam_teleport install

ENV SOFTHSM2_PATH "/usr/lib/softhsm/libsofthsm2.so"

# Install Rust
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
RUN curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal --default-toolchain $RUST_VERSION && \
    rustup --version && \
    cargo --version && \
    rustc --version && \
    rustup component add rustfmt clippy && \
    if [ "$BUILDARCH" = "amd64" ]; then rustup target add i686-unknown-linux-gnu arm-unknown-linux-gnueabihf aarch64-unknown-linux-gnu; fi && \
    cargo install cbindgen

USER root
VOLUME ["/go/src/github.com/gravitational/teleport"]
EXPOSE 6600 2379 2380
