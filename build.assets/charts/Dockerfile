ARG DEB_SOURCE

# Stage to download arch-specific Teleport debs from APT
FROM ubuntu:20.04 AS apt

ARG PACKAGE_NAME
ARG PACKAGE_VERSION
# Default to 30 minutes
ARG TIMEOUT=1800
ARG SLEEP_TIME=15

WORKDIR /tmp

# Add the Teleport APT repo
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install --no-install-recommends -y ca-certificates curl && \
    update-ca-certificates && \
    curl https://apt.releases.teleport.dev/gpg -o /usr/share/keyrings/teleport-archive-keyring.asc && \
    . /etc/os-release && \
    MAJOR_VERSION=$(echo ${PACKAGE_VERSION} | cut -d'.' -f 1) && \
    echo "deb [signed-by=/usr/share/keyrings/teleport-archive-keyring.asc] https://apt.releases.teleport.dev/${ID?} \
        ${VERSION_CODENAME?} stable/${MAJOR_VERSION}" > /etc/apt/sources.list.d/teleport.list

# Wait for the package to become available and download it
RUN END_TIME=$(( $(date +%s) + ${TIMEOUT} )) && \
    TRIMMED_VERSION=$(echo ${PACKAGE_VERSION} | cut -d'v' -f 2) && \
    TIMED_OUT=true && \
    while [ $(date +%s) -lt $END_TIME ]; do \
        # This will error on new major versions where the "stable/${MAJOR_VERSION}" component doesn't exist yet,
        # so we ignore it here.
        echo "Running apt update..."; \
        DEBIAN_FRONTEND=noninteractive apt-get update > /dev/null || true; \
        [ $(apt-cache madison ${PACKAGE_NAME} | grep $TRIMMED_VERSION | wc -l) -ge 1 ] && TIMED_OUT=false && break; \
        echo "Package not found yet, waiting another ${SLEEP_TIME} seconds..."; \
        sleep ${SLEEP_TIME}; \
    done && \
    [ $TIMED_OUT = true ] && echo "Timed out while looking for APT package \"${PACKAGE_NAME}\" matching $TRIMMED_VERSION" || \
    FULL_VERSION=$(apt-cache madison ${PACKAGE_NAME} | grep $TRIMMED_VERSION | cut -d'|' -f 2 | tr -d ' ' | head -n 1) && \
    echo "Found APT package, downloading \"${PACKAGE_NAME}=${FULL_VERSION}\"..." && \
    apt download ${PACKAGE_NAME}=$FULL_VERSION && \
    mv teleport*.deb /tmp/teleport.deb

# Stage to copy the local built deb
FROM alpine AS local

ARG DEB_PATH

COPY "$DEB_PATH" "/tmp/teleport.deb"

# Stage to aggregate/choose the deb source
# Dockerfile syntax does not support using a variable in copy's `--from=`
# argument, so this is a workaround.
FROM ${DEB_SOURCE} AS deb_source

# Stage to build the actual image
FROM ubuntu:20.04 AS teleport

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

FROM teleport AS teleport-fips
ENTRYPOINT ["/usr/bin/dumb-init", "teleport", "start", "-c", "/etc/teleport/teleport.yaml", "--fips"]
