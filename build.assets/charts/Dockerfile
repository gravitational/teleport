FROM ubuntu:18.04

# Install dumb-init and ca-certificate. The dumb-init package is to ensure
# signals and orphaned processes are are handled correctly. The ca-certificate
# package is installed because the base Ubuntu image does not come with any
# certificate authorities.
#
# Note that /var/lib/apt/lists/* is cleaned up in the same RUN command as
# "apt-get update" to reduce the size of the image.
RUN apt-get update && apt-get upgrade -y && \
    apt-get install --no-install-recommends -y \
    dumb-init \
    ca-certificates \
    && update-ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Bundle "teleport", "tctl", and "tsh" binaries into image.
ADD teleport /usr/local/bin/teleport
ADD tctl /usr/local/bin/tctl
ADD tsh /usr/local/bin/tsh

# By setting this entry point, we expose make target as command.
ENTRYPOINT ["/usr/bin/dumb-init", "teleport", "start", "-c", "/etc/teleport/teleport.yaml"]
