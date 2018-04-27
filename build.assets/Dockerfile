# This Dockerfile makes the "build box": the container used to build
# official releases of Teleport and its documentation
FROM quay.io/gravitational/buildbox-base

ARG UID
ARG GID

COPY pam/pam_teleport.so /lib/x86_64-linux-gnu/security
COPY pam/teleport-acct-failure /etc/pam.d
COPY pam/teleport-session-failure /etc/pam.d
COPY pam/teleport-success /etc/pam.d

RUN apt-get install -q -y libpam-dev

RUN (groupadd jenkins --gid=$GID -o && useradd jenkins --uid=$UID --gid=$GID --create-home --shell=/bin/sh ;\
     mkdir -p /var/lib/teleport && chown -R jenkins /var/lib/teleport)

# get etcd
RUN (curl -L https://github.com/coreos/etcd/releases/download/v3.0.16/etcd-v3.0.16-linux-amd64.tar.gz | tar -xz ;\
     cp etcd-v3.0.16-linux-amd64/etcd* /bin/ ;\
     apt-get install -y net-tools tree)

# Install Golang:
RUN mkdir -p /opt && cd /opt && curl https://storage.googleapis.com/golang/go1.9.2.linux-amd64.tar.gz | tar xz;\
    mkdir -p /gopath/src/github.com/gravitational/teleport;\
    chmod a+w /gopath;\
    chmod a+w /var/lib;\
    chmod a-w /

ENV GOPATH="/gopath" \
    GOROOT="/opt/go" \
    PATH="$PATH:/opt/go/bin:/gopath/bin:/gopath/src/github.com/gravitational/teleport/build"

VOLUME ["/gopath/src/github.com/gravitational/teleport"]
EXPOSE 6600 2379 2380
