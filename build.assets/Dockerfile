# This Dockerfile makes the "build box": the container used to build official
# releases of Teleport and its documentation.

# Use CentOS 7 as a base to get an older glibc version (glibc 2.18).
# This version is compatible with just about every distro other than CentOS 6.
# Using a newer base image will build against a newer glibc, which creates a
# runtime requirement for the host to have newer glibc too.
FROM centos:7

COPY profile /etc/profile

ENV LANGUAGE="en_US.UTF-8" \
    LANG="en_US.UTF-8" \
    LC_ALL="en_US.UTF-8" \
    LC_CTYPE="en_US.UTF-8"

RUN yum -y update && \
    yum -y install bcc-devel curl gcc git glibc-devel make net-tools pam-devel tar tree zip && \
    yum -y clean all

ARG UID
ARG GID
RUN (groupadd ci --gid=$GID -o && useradd ci --uid=$UID --gid=$GID --create-home --shell=/bin/sh && \
     mkdir -p -m0700 /var/lib/teleport && chown -R ci /var/lib/teleport)

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

ENV GOPATH="/go" \
    GOROOT="/opt/go" \
    PATH="$PATH:/opt/go/bin:/go/bin:/go/src/github.com/gravitational/teleport/build"

# Install meta-linter.
RUN (curl -L https://github.com/golangci/golangci-lint/releases/download/v1.24.0/golangci-lint-1.24.0-$(go env GOOS)-$(go env GOARCH).tar.gz | tar -xz && \
     cp golangci-lint-1.24.0-$(go env GOOS)-$(go env GOARCH)/golangci-lint /bin/ && \
     rm -r golangci-lint*)

# Install shellcheck.
RUN (curl -L https://github.com/koalaman/shellcheck/releases/download/v0.7.1/shellcheck-v0.7.1.$(go env GOOS).$(uname -m).tar.xz | tar -xJ && \
     cp shellcheck-v0.7.1/shellcheck /bin/ && \
     rm -r shellcheck*)

# Install PAM module and policies for testing.
COPY pam/ /opt/pam_teleport/
RUN make -C /opt/pam_teleport install

VOLUME ["/go/src/github.com/gravitational/teleport"]
EXPOSE 6600 2379 2380
