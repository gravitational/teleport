ARG BUILDBOX
ARG BASE_IMAGE=gcr.io/distroless/static-debian12

# BUILDPLATFORM is provided by Docker/buildx
FROM --platform=$BUILDPLATFORM $BUILDBOX as builder

WORKDIR /go/src/github.com/gravitational/teleport/

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# We have to copy the API before `go mod download` because go.mod has a replace directive for it
COPY api/ api/

RUN go mod download

# Copy in code
COPY version.go constants.go ./
COPY lib/ ./lib/
COPY build.assets/images.mk build.assets/arch.mk ./build.assets/
COPY integrations/lib/ ./integrations/lib/
COPY integrations/access/ ./integrations/access/

RUN pwd && ls -la

ARG TARGETOS
ARG TARGETARCH
ARG ACCESS_PLUGIN
ARG VERSION

WORKDIR /go/src/github.com/gravitational/teleport/integrations/access/$ACCESS_PLUGIN

# Build the program. We rely on golang's cross-compilation capabilities for multiarch building.
RUN make build/teleport-$ACCESS_PLUGIN VERSION=$VERSION

# Create the image with the build operator on the $TARGETPLATFORM
# TARGETPLATFORM is provided by Docker/buildx
FROM --platform=$TARGETPLATFORM $BASE_IMAGE
WORKDIR /
ARG ACCESS_PLUGIN
COPY --from=builder /go/src/github.com/gravitational/teleport/integrations/access/$ACCESS_PLUGIN/build/teleport-$ACCESS_PLUGIN /usr/local/bin/teleport-plugin

ENTRYPOINT ["/usr/local/bin/teleport-plugin"]
