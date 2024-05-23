ARG BUILDBOX
ARG BASE_IMAGE=gcr.io/distroless/static-debian12

# BUILDPLATFORM is provided by Docker/buildx
FROM --platform=$BUILDPLATFORM $BUILDBOX as builder

WORKDIR /go/src/github.com/gravitational/teleport/

# Copy dependencies and go.mod
COPY api/ api/
COPY version.go constants.go go.mod go.sum ./
COPY lib/ ./lib/
COPY build.assets/images.mk build.assets/arch.mk ./build.assets/
COPY integrations/lib/ ./integrations/lib/

COPY integrations/event-handler/go.mod ./integrations/event-handler/go.mod
COPY integrations/event-handler/go.sum ./integrations/event-handler/go.sum

RUN cd integrations/event-handler && go mod download

COPY integrations/event-handler ./integrations/event-handler

ARG TARGETOS
ARG TARGETARCH
ARG GITREF
ARG VERSION

WORKDIR /go/src/github.com/gravitational/teleport/integrations/event-handler

# Build the program. We rely on golang's cross-compilation capabilities for multiarch building.
RUN make build GITREF=$GITREF VERSION=$VERSION

# Create the image with the build operator on the $TARGETPLATFORM
# TARGETPLATFORM is provided by Docker/buildx
FROM --platform=$TARGETPLATFORM $BASE_IMAGE
WORKDIR /
COPY --from=builder /go/src/github.com/gravitational/teleport/integrations/event-handler/build/teleport-event-handler /usr/local/bin/teleport-event-handler

ENTRYPOINT ["/usr/local/bin/teleport-event-handler"]
