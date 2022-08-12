ARG DEB_SOURCE

# Stage to download arch-specific Teleport debs from APT
FROM ubuntu:20.04 AS download

WORKDIR /tmp

# Install dependencies.
RUN apk --update --no-cache add curl

# Download the repo signing key
RUN curl https://apt.releases.teleport.dev/gpg \
  -o /tmp/teleport-archive-keyring.asc

# Stage to copy the local built deb
FROM alpine AS local_copy

ARG DEB_PATH

COPY "$DEB_PATH" "/tmp/teleport.deb"

# Stage to aggregate/choose the deb source
FROM ${DEB_SOURCE} as deb_source

# Stage to build the actual image
FROM ubuntu:20.04 AS teleport

ARG MAJOR_VERSION
ARG PACKAGE_NAME

# Copy the deb archive
COPY --from=deb_source /tmp/teleport.deb /tmp/teleport.deb

# Install dumb-init and ca-certificates. The dumb-init package is to ensure
# signals and orphaned processes are are handled correctly. The ca-certificate
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
# * inetutils-telnet
#   * telnet
# * netcat
#   * netcat
# * tcpdump
#   * tcpdump
# * busybox (see "busybox --list" for all provided utils)
#   * less
#   * nslookup
#   * vi
#   * wget

    # Update the base image
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get upgrade -y && \
    # Install dependencies
    DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y ca-certificates dumb-init libelf1 && \
    update-ca-certificates && \
    # Install tools
    DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y net-tools iputils-ping inetutils-telnet netcat tcpdump busybox && \
    busybox --install -s && \
    # Install Teleport
    dpkg -i /tmp/teleport.deb && \
    # Cleanup
    apt-get -y clean && \
    rm -rf /var/lib/apt/lists/*

# By setting this entry point, we expose make target as command.
ENTRYPOINT ["/usr/bin/dumb-init", "teleport", "start", "-c", "/etc/teleport/teleport.yaml"]

FROM teleport as teleport-fips
ENTRYPOINT ["/usr/bin/dumb-init", "teleport", "start", "-c", "/etc/teleport/teleport.yaml", "--fips"]
