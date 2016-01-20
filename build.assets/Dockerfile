FROM debian:jessie

ENV DEBIAN_FRONTEND noninteractive

ADD locale.gen /etc/locale.gen
ADD profile /etc/profile

RUN (apt-get clean \
    && apt-key update \
	&& apt-get -q -y update --fix-missing \
    && apt-get -q -y update \
	&& apt-get install -q -y apt-utils \
	&& apt-get install -q -y less \
	&& apt-get install -q -y locales) ; 

# Set locale to en_US.UTF-8
RUN (locale-gen \
	&& locale-gen en_US.UTF-8 \
	&& dpkg-reconfigure locales)

RUN (apt-get update && apt-get install -q -y bridge-utils \
     kmod \
     iptables \
     libdevmapper1.02.1 \
     libsqlite3-0 \
     e2fsprogs \
     libncurses5 \
     curl \
     make \
     git \
     libc6-dev \
     gcc \
     tar \
     gzip \
     dbus); \
    apt-get -y autoclean; apt-get -y clean

RUN (mkdir -p /opt && cd /opt && curl https://storage.googleapis.com/golang/go1.5.3.linux-amd64.tar.gz | tar xz)
RUN (mkdir -p /gopath/src/github.com/gravitational/teleport)

ENV LANGUAGE="en_US.UTF-8" \
    LANG="en_US.UTF-8" \
    LC_ALL="en_US.UTF-8" \
    LC_CTYPE="en_US.UTF-8" \
    GOPATH="/gopath" \
    GOROOT="/opt/go" \
    PATH="$PATH:/opt/go/bin:/gopath/bin"

VOLUME ["/gopath/src/github.com/gravitational/teleport"]
