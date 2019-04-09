ARG TELEPORT_VERSION
FROM quay.io/gravitational/teleport-ent:${TELEPORT_VERSION}

COPY rootfs/ /

ENTRYPOINT ["/usr/bin/dumb-init", "/scripts/teleport.sh"]