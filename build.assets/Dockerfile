# This Dockerfile makes the "build box": the container used to build official
# releases of Teleport and its documentation.
FROM quay.io/gravitational/buildbox-base:1.0

ARG UID
ARG GID

COPY pam/pam_teleport.so /lib/x86_64-linux-gnu/security
COPY pam/teleport-acct-echo /etc/pam.d
COPY pam/teleport-acct-failure /etc/pam.d
COPY pam/teleport-success /etc/pam.d
COPY pam/teleport-session-failure /etc/pam.d
COPY pam/teleport-session-environment /etc/pam.d

RUN apt-get update; apt-get install -q -y libpam-dev libc6-dev-i386 net-tools tree

RUN (groupadd jenkins --gid=$GID -o && useradd jenkins --uid=$UID --gid=$GID --create-home --shell=/bin/sh ;\
     mkdir -p /var/lib/teleport && chown -R jenkins /var/lib/teleport)

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
