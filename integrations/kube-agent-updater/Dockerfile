ARG BUILDBOX
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

# Compiler package should use host-triplet-agnostic name (i.e. "x86-64-linux-gnu-gcc" instead of "gcc")
#  in most cases, to avoid issues on systems with multiple versions of gcc (i.e. buildboxes)
# TARGETOS and TARGETARCH are provided by Docker/buildx, but must be explicitly listed here
ARG COMPILER_NAME TARGETOS TARGETARCH

# Build the program
# CGO is required for github.com/gravitational/teleport/lib/system
RUN echo "Targeting $TARGETOS/$TARGETARCH with CC=$COMPILER_NAME" && \
    CGO_ENABLED=1 CC=$COMPILER_NAME GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -a -o /go/bin/teleport-kube-agent-updater github.com/gravitational/teleport/integrations/kube-agent-updater/cmd/teleport-kube-agent-updater

# Create the image with the build operator on the $TARGETPLATFORM
# TARGETPLATFORM is provided by Docker/buildx
FROM --platform=$TARGETPLATFORM gcr.io/distroless/cc
WORKDIR /
COPY --from=builder /go/bin/teleport-kube-agent-updater .

ENTRYPOINT ["/teleport-kube-agent-updater"]
