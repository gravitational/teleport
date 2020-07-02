# This Dockerfile makes the "build box": the container used to build official
# releases of Teleport and its documentation.
FROM debian:stretch

ADD locale.gen /etc/locale.gen

ENV LANGUAGE="en_US.UTF-8" \
    DEBIAN_FRONTEND="noninteractive" \
    LANG="en_US.UTF-8" \
    LC_ALL="en_US.UTF-8" \
    LC_CTYPE="en_US.UTF-8"

RUN apt-get update -y --fix-missing && \
    apt-get -q -y upgrade && \
    apt-get install -q -y apt-utils curl gcc git gzip libc6-dev libpam-dev libsqlite3-0 locales make net-tools tar tree zip && \
    locale-gen && \
    locale-gen en_US.UTF-8 && \
    dpkg-reconfigure locales && \
    apt-get -y autoclean && apt-get -y clean
ADD profile /etc/profile

ARG UID
ARG GID

COPY pam/pam_teleport.so /lib/x86_64-linux-gnu/security
COPY pam/teleport-acct-echo /etc/pam.d
COPY pam/teleport-acct-failure /etc/pam.d
COPY pam/teleport-success /etc/pam.d
COPY pam/teleport-session-failure /etc/pam.d
COPY pam/teleport-session-environment /etc/pam.d

RUN (groupadd jenkins --gid=$GID -o && useradd jenkins --uid=$UID --gid=$GID --create-home --shell=/bin/sh ;\
     mkdir -p -m0700 /var/lib/teleport && chown -R jenkins /var/lib/teleport)

# Install etcd.
RUN (curl -L https://github.com/coreos/etcd/releases/download/v3.3.9/etcd-v3.3.9-linux-amd64.tar.gz | tar -xz ;\
     cp etcd-v3.3.9-linux-amd64/etcd* /bin/)

# Install Go.
ARG RUNTIME
RUN mkdir -p /opt && cd /opt && curl https://storage.googleapis.com/golang/$RUNTIME.linux-amd64.tar.gz | tar xz;\
    mkdir -p /gopath/src/github.com/gravitational/teleport;\
    chmod a+w /gopath;\
    chmod a+w /var/lib;\
    chmod a-w /

ENV GOPATH="/gopath" \
    GOROOT="/opt/go" \
    PATH="$PATH:/opt/go/bin:/gopath/bin:/gopath/src/github.com/gravitational/teleport/build"

# Install meta-linter.
RUN (curl -L https://github.com/golangci/golangci-lint/releases/download/v1.24.0/golangci-lint-1.24.0-$(go env GOOS)-$(go env GOARCH).tar.gz | tar -xz ;\
     cp golangci-lint-1.24.0-$(go env GOOS)-$(go env GOARCH)/golangci-lint /bin/ ;\
     rm -r golangci-lint*)

VOLUME ["/gopath/src/github.com/gravitational/teleport"]
EXPOSE 6600 2379 2380
