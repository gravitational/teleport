ARG BASE_IMAGE=gcr.io/distroless/static-debian12

# BUILDPLATFORM is provided by Docker/buildx
# Extract the built tarball to reduce the final image size
FROM --platform=${BUILDPLATFORM} alpine:3.20.0 as extractor

WORKDIR /extraction
ARG TARGETARCH
COPY *${TARGETARCH}*.tar.gz /plugin.tar.gz

## ***** for debug
COPY *${TARGETARCH}*.tar.gz .
RUN pwd && ls -laht . && echo *${TARGETARCH}*.tar.gz \
    && tar -C /tmp -xzvf *${TARGETARCH}*.tar.gz && ls -laht /tmp \
    && find /tmp -type f -executable -name 'teleport-*' -exec file {} \;
## *****

RUN tar -xzvf /plugin.tar.gz && \
    find . -type f -executable -name 'teleport-*' -exec mv {} /teleport-plugin \;

# Create the image with the build operator on the $TARGETPLATFORM
# TARGETPLATFORM is provided by Docker/buildx
FROM --platform=${TARGETPLATFORM} ${BASE_IMAGE}
WORKDIR /
COPY --from=extractor /teleport-plugin /usr/local/bin/teleport-plugin

ENTRYPOINT ["/usr/local/bin/teleport-plugin"]
