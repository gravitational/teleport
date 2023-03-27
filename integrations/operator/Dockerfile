ARG BUILDBOX
# BUILDPLATFORM is provided by Docker/buildx
FROM --platform=$BUILDPLATFORM $BUILDBOX as builder

WORKDIR /go/src/github.com/gravitational/teleport

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# We have to copy the API before `go mod download` because go.mod has a replace directive for it
COPY api/ api/

# Download and Cache dependencies before building and copying source
# This will prevent re-downloading the operator's dependencies if they have not changed as this
# `run` layer will be cached
RUN go mod download

COPY *.go ./
COPY lib/ lib/
COPY gen/ gen/
COPY integrations/operator/apis/ integrations/operator/apis/
COPY integrations/operator/controllers/ integrations/operator/controllers/
COPY integrations/operator/sidecar/ integrations/operator/sidecar/
COPY integrations/operator/main.go integrations/operator/main.go
COPY integrations/operator/namespace.go integrations/operator/namespace.go

# Compiler package should use host-triplet-agnostic name (i.e. "x86-64-linux-gnu-gcc" instead of "gcc")
#  in most cases, to avoid issues on systems with multiple versions of gcc (i.e. buildboxes)
# TARGETOS and TARGETARCH are provided by Docker/buildx, but must be explicitly listed here
ARG COMPILER_NAME TARGETOS TARGETARCH

# Build the program
# CGO is required for github.com/gravitational/teleport/lib/system
RUN echo "Targeting $TARGETOS/$TARGETARCH with CC=$COMPILER_NAME" && \
    CGO_ENABLED=1 CC=$COMPILER_NAME GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -a -o /go/bin/teleport-operator github.com/gravitational/teleport/integrations/operator

# Create the image with the build operator on the $TARGETPLATFORM
# TARGETPLATFORM is provided by Docker/buildx
FROM --platform=$TARGETPLATFORM gcr.io/distroless/cc
WORKDIR /
COPY --from=builder /go/bin/teleport-operator .

ENTRYPOINT ["/teleport-operator"]
