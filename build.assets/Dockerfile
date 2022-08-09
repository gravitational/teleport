# This Dockerfile makes the "build box": the container used to build official
# releases of Teleport and its documentation.

# Use Ubuntu 18.04 as base to get an older glibc version.
# Using a newer base image will build against a newer glibc, which creates a
# runtime requirement for the host to have newer glibc too. For example,
# teleport built on any newer Ubuntu version will not run on Centos 7 because
# of this.

ARG RUST_VERSION

## LIBFIDO2 ###################################################################

# Build libfido2 separately for isolation, speed and flexibility.
FROM buildpack-deps:18.04 AS libfido2

RUN apt-get update && \
    apt-get install -y --no-install-recommends cmake && \
    rm -rf /var/lib/apt/lists/*

# Install libudev-zero.
# libudev-zero replaces systemd's libudev
RUN git clone --depth=1 https://github.com/illiliti/libudev-zero.git -b 1.0.1 && \
    cd libudev-zero && \
    [ "$(git rev-parse HEAD)" = "4154cf252c17297f98a8ca33693ead003b4509da" ] && \
    make install-static && \
    make clean

# Install libcbor.
RUN git clone --depth=1 https://github.com/PJK/libcbor.git -b v0.9.0 && \
    cd libcbor && \
    [ "$(git rev-parse HEAD)" = "58b3319b8c3ec15171cb00f01a3a1e9d400899e1" ] && \
    cmake \
        -DCBOR_CUSTOM_ALLOC=ON \
        -DCMAKE_BUILD_TYPE=Release \
        -DCMAKE_POSITION_INDEPENDENT_CODE=ON \
        -DWITH_EXAMPLES=OFF . && \
    make && \
    make install && \
    make clean

# Install libfido2.
# Depends on libcbor, libssl-dev, zlib1g-dev and libudev.
RUN git clone --depth=1 https://github.com/Yubico/libfido2.git -b 1.11.0 && \
    cd libfido2 && \
    [ "$(git rev-parse HEAD)" = "e61379ff0a27277fbe0aca29ccc34ff93c57b359" ] && \    
    CFLAGS=-pthread cmake \
        -DBUILD_EXAMPLES=OFF \
        -DBUILD_MANPAGES=OFF \
        -DBUILD_TOOLS=OFF \
        -DCMAKE_BUILD_TYPE=Release . && \
    make && \
    make install && \
    make clean

## LIBBPF #####################################################################

FROM buildpack-deps:18.04 AS libbpf

# Install libbpf
RUN apt-get update -y --fix-missing && \
    apt-get -q -y upgrade && \
    apt-get install -q -y --no-install-recommends \
        libelf-dev

ARG LIBBPF_VERSION
RUN mkdir -p /opt && cd /opt && curl -L https://github.com/gravitational/libbpf/archive/refs/tags/v${LIBBPF_VERSION}.tar.gz | tar xz && \
    cd /opt/libbpf-${LIBBPF_VERSION}/src && \
    make && \
    make install

## BUILDBOX ###################################################################

FROM ubuntu:18.04 AS buildbox

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
        build-essential \
        ca-certificates \
        clang-10 \
        clang-format-10 \
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
        libudev-dev \
        llvm-10 \
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
        shellcheck \
        softhsm2 \
        sudo \
        tree \
        unzip \
        zip \
        zlib1g-dev \
        xauth \
        && \
    pip3 --no-cache-dir install yamllint && \
    dpkg-reconfigure locales && \
    apt-get -y clean && \
    rm -rf /var/lib/apt/lists/*

# Install gcloud SDK and Firestore emulator.
ENV PATH="$PATH:/opt/google-cloud-sdk/bin"
RUN (curl -sSL https://sdk.cloud.google.com | bash -s -- --install-dir=/opt --disable-prompts) && \
    gcloud components install cloud-firestore-emulator beta && \
    rm -rf /opt/google-cloud-sdk/.install/.backup

ARG UID
ARG GID
RUN (groupadd ci --gid=$GID -o && useradd ci --uid=$UID --gid=$GID --create-home --shell=/bin/sh && \
     mkdir -p -m0700 /var/lib/teleport && chown -R ci /var/lib/teleport)

# Install etcd.
RUN (curl -L https://github.com/coreos/etcd/releases/download/v3.3.9/etcd-v3.3.9-linux-${BUILDARCH}.tar.gz | tar -xz && \
     cp etcd-v3.3.9-linux-${BUILDARCH}/etcd* /bin/ && \
     rm -rf etcd-v3.3.9-linux-${BUILDARCH})

# Install Go.
ARG GOLANG_VERSION
RUN mkdir -p /opt && cd /opt && curl https://storage.googleapis.com/golang/$GOLANG_VERSION.linux-${BUILDARCH}.tar.gz | tar xz && \
    mkdir -p /go/src/github.com/gravitational/teleport && \
    chmod a+w /go && \
    chmod a+w /var/lib && \
    chmod a-w /
ENV GOPATH="/go" \
    GOROOT="/opt/go" \
    PATH="$PATH:/opt/go/bin:/go/bin:/go/src/github.com/gravitational/teleport/build"

# Install addlicense
RUN go install github.com/google/addlicense@v1.0.0

# Install golangci-lint.
RUN (curl -L https://github.com/golangci/golangci-lint/releases/download/v1.46.0/golangci-lint-1.46.0-$(go env GOOS)-$(go env GOARCH).tar.gz | tar -xz && \
     cp golangci-lint-1.46.0-$(go env GOOS)-$(go env GOARCH)/golangci-lint /bin/ && \
     rm -r golangci-lint*)

# Install helm.
RUN (mkdir -p helm-tarball && curl -L https://get.helm.sh/helm-v3.5.2-$(go env GOOS)-$(go env GOARCH).tar.gz | tar -C helm-tarball -xz && \
     cp helm-tarball/$(go env GOOS)-$(go env GOARCH)/helm /bin/ && \
     rm -r helm-tarball*)
RUN helm plugin install https://github.com/vbehar/helm3-unittest && \
    mkdir -p /home/ci/.local/share/helm && \
    cp -r /root/.local/share/helm/plugins /home/ci/.local/share/helm && \
    chown -R ci /home/ci/.local/share/helm && \
    HELM_PLUGINS=/home/ci/.local/share/helm/plugins helm plugin list

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
     chmod -R a+xr /usr/local/bin/protoc && \
     rm /tmp/${PROTOC_TARBALL})
RUN (git clone https://github.com/gogo/protobuf.git --branch ${GOGO_PROTO_TAG} --depth 1 ${GOPATH}/src/github.com/gogo/protobuf && \
     cd ${GOPATH}/src/github.com/gogo/protobuf && \
     make install && \
     make clean)

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
    if [ "$BUILDARCH" = "amd64" ]; then rustup target add i686-unknown-linux-gnu arm-unknown-linux-gnueabihf aarch64-unknown-linux-gnu; fi

# Switch back to root for the remaining instructions and keep it as the default
# user.
USER root

COPY --from=libbpf /usr/include/bpf /usr/include/bpf
COPY --from=libbpf /usr/lib64/ /usr/lib64/
RUN cd /usr/local/lib && ldconfig

# Copy libfido2 libraries.
# Do this last to take better advantage of the multi-stage build.
COPY --from=libfido2 /usr/local/include/ /usr/local/include/
COPY --from=libfido2 /usr/local/lib/pkgconfig/ /usr/local/lib/pkgconfig/
COPY --from=libfido2 \
    /usr/local/lib/libcbor.a \
    /usr/local/lib/libfido2.a \
    /usr/local/lib/libfido2.so.1.11.0 \
    /usr/local/lib/libudev.a \
    /usr/local/lib/
RUN cd /usr/local/lib && \
    ln -s libfido2.so.1.11.0 libfido2.so.1 && \
    ln -s libfido2.so.1 libfido2.so && \
    ldconfig
COPY pkgconfig/buildbox/ /

VOLUME ["/go/src/github.com/gravitational/teleport"]
EXPOSE 6600 2379 2380
