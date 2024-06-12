# Stage to build the image, without FIPS entrypoint argument
FROM ubuntu:20.04 AS teleport

# Install dumb-init and ca-certificates. The dumb-init package is to ensure
# signals and orphaned processes are are handled correctly. The ca-certificates
# package is installed because the base Ubuntu image does not come with any
# certificate authorities. libelf1 is a dependency introduced by Teleport 7.0.
#
# The below packages are provided for debug purposes. Installing them adds around
#  six megabytes to the image size. The packages include the following commands:
# * net-tools
#   * netstat
#   * ifconfig
#   * ipmaddr
#   * iptunnel
#   * mii-tool
#   * nameif
#   * plipconfig
#   * rarp
#   * route
#   * slattach
#   * arp
# * iputils-ping
#   * ping
#   * ping4
#   * ping6
# * netcat
#   * netcat
# * tcpdump
#   * tcpdump
# * busybox (see "busybox --list" for all provided utils)
#   * less
#   * nslookup
#   * vi
#   * wget

# Note that /var/lib/apt/lists/* is cleaned up in the same RUN command as
# "apt-get update" to reduce the size of the image.
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get upgrade -y && \
    # Install dependencies
    DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y ca-certificates dumb-init libelf1 && \
    # Install tools
    DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y net-tools iputils-ping netcat tcpdump busybox && \
    busybox --install -s && \
    update-ca-certificates && \
    # Cleanup
    apt-get -y clean && \
    rm -rf /var/lib/apt/lists/*

# DEB_PATH speficies the path to the Teleport .deb file under
# the build context. If not specified, DEB_BASE specifies the path
# prefix to the Teleport .deb file without the architecture and
# `.deb` on the end. DEB_BASE is used for multi-architecture builds.
# DEB_PATH is deprecated and will be removed.
ARG DEB_PATH
ARG DEB_BASE
ARG TARGETARCH

# Note --mount=target=/ctx which mounts the docker build context on /ctx
# providing access to the .deb file to install without needing to COPY it
# in, saving space in the image.
RUN --mount=target=/ctx \
    # Install Teleport
    dpkg -i /ctx/${DEB_PATH:-${DEB_BASE}_${TARGETARCH}.deb}

# Used to track whether a Teleport agent was installed using this method.
ENV TELEPORT_INSTALL_METHOD_DOCKERFILE=true

# By setting this entry point, we expose make target as command.
ENTRYPOINT ["/usr/bin/dumb-init", "teleport", "start", "-c", "/etc/teleport/teleport.yaml"]

# Stage to launch Teleport with the fips argument
FROM teleport AS teleport-fips
ENTRYPOINT ["/usr/bin/dumb-init", "teleport", "start", "-c", "/etc/teleport/teleport.yaml", "--fips"]
