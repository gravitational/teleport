FROM ubuntu:20.04

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

# Note that /var/lib/apt/lists/* is cleaned up in the same RUN command as
# "apt-get update" to reduce the size of the image.
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get upgrade -y && \
    DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y ca-certificates dumb-init libelf1 && \
    DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y net-tools iputils-ping inetutils-telnet netcat tcpdump busybox && \
    busybox --install -s && \
    update-ca-certificates && \
    apt-get -y clean && \
    rm -rf /var/lib/apt/lists/*

# Bundle "teleport", "tctl", and "tsh" binaries into image.
COPY teleport /usr/local/bin/teleport
COPY tctl /usr/local/bin/tctl
COPY tsh /usr/local/bin/tsh

# By setting this entry point, we expose make target as command.
ENTRYPOINT ["/usr/bin/dumb-init", "teleport", "start", "-c", "/etc/teleport/teleport.yaml"]
