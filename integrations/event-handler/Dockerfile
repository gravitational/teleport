ARG BASE_IMAGE=gcr.io/distroless/static-debian12

# BUILDPLATFORM is provided by Docker/buildx
# Extract the built tarball to reduce the final image size
FROM --platform=${BUILDPLATFORM} alpine:3.20.0 as extractor

ARG TARGETARCH
ARG TARGETOS
ARG ARTIFACT_NAME=plugin    # Should be something like 'access-slack' or 'event-handler'
ARG TELEPORT_VERSION=0.0.0  # Should be the version without 'v' prefix
ARG FILENAME=teleport-${ARTIFACT_NAME}-v${TELEPORT_VERSION}-${TARGETOS}-${TARGETARCH}-bin.tar.gz # Optional override

WORKDIR /extraction
COPY *${TARGETARCH}*.tar.gz /plugin.tar.gz
COPY "${FILENAME}" /plugin.tar.gz

RUN tar -xzvf /plugin.tar.gz && \
    find . -type f -executable -name 'teleport-*' -exec mv {} /teleport-plugin \;

# Create the image with the build operator on the $TARGETPLATFORM
# TARGETPLATFORM is provided by Docker/buildx
FROM --platform=${TARGETPLATFORM} ${BASE_IMAGE}
WORKDIR /
COPY --from=extractor /teleport-plugin /usr/local/bin/teleport-plugin

ENTRYPOINT ["/usr/local/bin/teleport-plugin"]
