ARG BUILDBOX
ARG BASE_IMAGE=gcr.io/distroless/static-debian11
# BUILDPLATFORM is provided by Docker/buildx
FROM --platform=$BUILDPLATFORM $BUILDBOX as builder

WORKDIR /go/src/github.com/gravitational/teleport/integrations/kube-agent-updater

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Download and Cache dependencies before building and copying source
# This will prevent re-downloading the operator's dependencies if they have not changed as this
# `run` layer will be cached
RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

ARG TARGETOS
ARG TARGETARCH

# Build the program. We rely on golang's cross-compilation capabilities for multiarch building.
RUN echo "Targeting $TARGETOS/$TARGETARCH" && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH CGO_ENABLED=0 \
    go build -a -o /go/bin/teleport-kube-agent-updater github.com/gravitational/teleport/integrations/kube-agent-updater/cmd/teleport-kube-agent-updater

# Create the image with the build operator on the $TARGETPLATFORM
# TARGETPLATFORM is provided by Docker/buildx
FROM --platform=$TARGETPLATFORM $BASE_IMAGE
WORKDIR /
COPY --from=builder /go/bin/teleport-kube-agent-updater .

ENTRYPOINT ["/teleport-kube-agent-updater"]
