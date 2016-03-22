# This Dockerfile makes the "build box": the container used to build
# official releases of Teleport and its documentation
FROM debian:jessie

ARG UID
ARG GID

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

RUN (apt-get update && apt-get install -q -y \
         libsqlite3-0 \
         curl \
         make \
         git \
         libc6-dev \
         gcc \
         tar \
         gzip \
         python \
         python-pip \
         libyaml-dev \
         python-dev) ;\
    pip install click==4.1 recommonmark mkdocs  markdown-include ;\
    apt-get -y autoclean; apt-get -y clean

# Install Golang:
RUN (mkdir -p /opt && cd /opt && curl https://storage.googleapis.com/golang/go1.5.3.linux-amd64.tar.gz | tar xz;\
     mkdir -p /gopath/src/github.com/gravitational/teleport;\
     chmod a+w /gopath;\
     chmod a+w /var/lib)

RUN groupadd jenkins --gid=$GID && useradd jenkins --uid=$UID --gid=$GID --create-home --shell=/bin/sh
RUN (mkdir -p /var/lib/teleport && chown -R jenkins /var/lib/teleport)

ENV LANGUAGE="en_US.UTF-8" \
    LANG="en_US.UTF-8" \
    LC_ALL="en_US.UTF-8" \
    LC_CTYPE="en_US.UTF-8" \
    GOPATH="/gopath" \
    GOROOT="/opt/go" \
    PATH="$PATH:/opt/go/bin:/gopath/bin" \
    GO15VENDOREXPERIMENT="1"

VOLUME ["/gopath/src/github.com/gravitational/teleport"]
