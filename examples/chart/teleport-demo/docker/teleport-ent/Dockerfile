ARG TELEPORT_VERSION
FROM quay.io/gravitational/teleport-ent:${TELEPORT_VERSION}
RUN apt-get update && \
    apt-get -y install sudo

COPY rootfs/ /

ENTRYPOINT ["/usr/bin/dumb-init", "/scripts/teleport.sh"]
