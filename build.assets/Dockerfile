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

# Install packages.
# We install curl first to allow setting up the Google SDK as part of the same layer.
RUN apt-get update -y --fix-missing && \
    apt-get -q -y upgrade && \
    apt-get install -y --no-install-recommends apt-utils ca-certificates curl gnupg && \
    echo "deb [signed-by=/usr/share/keyrings/cloud.google.gpg] http://packages.cloud.google.com/apt cloud-sdk main" | tee -a /etc/apt/sources.list.d/google-cloud-sdk.list && \
    curl https://packages.cloud.google.com/apt/doc/apt-key.gpg | apt-key --keyring /usr/share/keyrings/cloud.google.gpg add - && \
    apt-get -y update && \
    apt-get install -q -y --no-install-recommends \
        apt-utils \
        clang-10 \
        clang-format-10 \
        curl \
        gcc \
        gcc-multilib \
        git \
        google-cloud-sdk \
        google-cloud-sdk-firestore-emulator \
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
        python-pip \
        pkg-config \
        shellcheck \
        softhsm2 \
        tar \
        tree \
        unzip \
        zip \
        zlib1g-dev \
        && \
    pip --no-cache-dir install yamllint && \
    dpkg-reconfigure locales && \
    apt-get -y clean && \
    rm -rf /var/lib/apt/lists/*

ARG UID
ARG GID
RUN (groupadd ci --gid=$GID -o && useradd ci --uid=$UID --gid=$GID --create-home --shell=/bin/sh && \
     mkdir -p -m0700 /var/lib/teleport && chown -R ci /var/lib/teleport)

# Install Rust
#
# Rust installation based on official rust image Dockerfile here:
#   https://github.com/rust-lang/docker-rust/blob/master/1.56.0/bullseye/Dockerfile
#
# The original Rust docker image uses a script to install `rustup`, and from
# there rustc and associated tools.
#
# Rather than execute an arbitrary `rustup` installation script, we are cherry-
# picking the appropriate files off the official docker image and then installing
# the extra tooling/targets we need.

 ENV RUSTUP_HOME=/usr/local/rustup \
     CARGO_HOME=/usr/local/cargo \
     PATH=/usr/local/cargo/bin:$PATH \
     RUST_VERSION=1.56.0

 COPY --from=rust:1.56.0 /usr/local/rustup /usr/local/rustup
 COPY --from=rust:1.56.0 /usr/local/cargo /usr/local/cargo
 RUN set -eux \
     rustup component add rustfmt clippy; \
     chmod -R a+w $RUSTUP_HOME $CARGO_HOME; \
     rustup --version; \
     cargo --version; \
     rustup target add i686-unknown-linux-gnu; \
     rustup target add arm-unknown-linux-gnueabihf; \
     rustup target add aarch64-unknown-linux-gnu; \
     rustup target list | grep installed; \
     rustc --version;

# Install etcd.
RUN (curl -L https://github.com/coreos/etcd/releases/download/v3.3.9/etcd-v3.3.9-linux-amd64.tar.gz | tar -xz && \
     cp etcd-v3.3.9-linux-amd64/etcd* /bin/)

# Install Go.
ARG RUNTIME
RUN mkdir -p /opt && cd /opt && curl https://storage.googleapis.com/golang/$RUNTIME.linux-amd64.tar.gz | tar xz && \
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
ARG PROTOC_PLATFORM
ARG GOGO_PROTO_TAG

ENV PROTOC_TARBALL protoc-${PROTOC_VER}-${PROTOC_PLATFORM}.zip
ENV GOGOPROTO_ROOT ${GOPATH}/src/github.com/gogo/protobuf

RUN (curl -L -o /tmp/${PROTOC_TARBALL} https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VER}/${PROTOC_TARBALL} && \
     cd /tmp && unzip /tmp/${PROTOC_TARBALL} -d /usr/local && \
     rm /tmp/${PROTOC_TARBALL})
RUN (git clone https://github.com/gogo/protobuf.git ${GOPATH}/src/github.com/gogo/protobuf && go install golang.org/x/tools/cmd/goimports@latest && \
     cd ${GOPATH}/src/github.com/gogo/protobuf && \
     git reset --hard ${GOGO_PROTO_TAG} && \
     make install)

# Note: protoc reads the proto files from /vendor/.../teleport/api/vX rather than
# /api because protoc does not understand go modules, and reads vX as a directory.
ENV PROTO_INCLUDE "/usr/local/include":"/go/src/github.com/gravitational/teleport/vendor":"/go/src/github.com/gogo/protobuf/protobuf":"${GOGOPROTO_ROOT}":"${GOGOPROTO_ROOT}/protobuf"

# Install PAM module and policies for testing.
COPY pam/ /opt/pam_teleport/
RUN make -C /opt/pam_teleport install

ENV SOFTHSM2_PATH "/usr/lib/softhsm/libsofthsm2.so"

VOLUME ["/go/src/github.com/gravitational/teleport"]
EXPOSE 6600 2379 2380
