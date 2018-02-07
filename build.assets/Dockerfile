# This Dockerfile makes the "build box": the container used to build
# official releases of Teleport and its documentation
FROM debian:jessie

ARG UID
ARG GID

ENV DEBIAN_FRONTEND noninteractive

ADD locale.gen /etc/locale.gen
ADD profile /etc/profile

RUN (apt-get clean; \
	 apt-get -q -y update --fix-missing; \
     apt-get -q -y update; \
     apt-get -q -y upgrade; \
	 apt-get install -q -y apt-utils less locales)

# Set locale to en_US.UTF-8
RUN locale-gen; \
	locale-gen en_US.UTF-8 ;\
	dpkg-reconfigure locales

RUN apt-get install -q -y \
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
         python-dev \
         nginx \         
         zip; \         
      apt-get -y autoclean; apt-get -y clean

# Install mkDocs
RUN pip install click==6.1 recommonmark==0.4.0 markdown-include==0.5.1 mkdocs==0.16.1 Markdown==2.6.7

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

# Install SASS
RUN  apt-get install -q -y ruby-sass

ENV LANGUAGE="en_US.UTF-8" \
    LANG="en_US.UTF-8" \
    LC_ALL="en_US.UTF-8" \
    LC_CTYPE="en_US.UTF-8" \
    GOPATH="/gopath" \
    GOROOT="/opt/go" \
    PATH="$PATH:/opt/go/bin:/gopath/bin:/gopath/src/github.com/gravitational/teleport/build"

VOLUME ["/gopath/src/github.com/gravitational/teleport"]
EXPOSE 6600 2379 2380
