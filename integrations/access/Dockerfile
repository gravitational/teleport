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
COPY version.go constants.go gitref.go ./
COPY lib/ ./lib/
COPY build.assets/images.mk build.assets/arch.mk ./build.assets/
COPY integrations/lib/ ./integrations/lib/
COPY integrations/access/ ./integrations/access/

RUN pwd && ls -la

ARG TARGETOS
ARG TARGETARCH
ARG ACCESS_PLUGIN

WORKDIR /go/src/github.com/gravitational/teleport/integrations/access/discord

# Build the program. We rely on golang's cross-compilation capabilities for multiarch building.
RUN make build/teleport-discord

# Create the image with the build operator on the $TARGETPLATFORM
# TARGETPLATFORM is provided by Docker/buildx
FROM --platform=$TARGETPLATFORM $BASE_IMAGE
WORKDIR /
COPY --from=builder /go/src/github.com/gravitational/teleport/integrations/access/discord/build/teleport-discord /usr/local/bin/teleport-plugin

ENTRYPOINT ["/usr/local/bin/teleport-plugin"]
