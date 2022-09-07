# Cross-compile the operator binary
# This is significantly faster than emulation of $TARGETPLATFORM with buildkit
# See https://github.com/moby/buildkit/blob/master/docs/multi-platform.md for details
FROM --platform=$BUILDPLATFORM golang:1.18 as builder

# GCC Compiler package should be of the form "gcc-<GNU TARGETARCH>-linux-<ABI format>"
# Compiler package should use host-triplet-agnostic name (i.e. "x86-64-linux-gnu-gcc" instead of "gcc")
ARG TARGETOS TARGETARCH COMPILER_PACKAGE COMPILER_NAME

WORKDIR /workspace

# Install build dependencies
RUN export DEBIAN_FRONTEND=noninteractive && apt update && apt install -y "$COMPILER_PACKAGE"

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
COPY operator/apis/ operator/apis/
COPY operator/controllers/ operator/controllers/
COPY operator/sidecar/ operator/sidecar/
COPY operator/main.go operator/main.go
COPY operator/namespace.go operator/namespace.go

# CGO is required for github.com/gravitational/teleport/lib/system
RUN echo "Targeting $TARGETOS/$TARGETARCH with CC=$COMPILER_NAME" && \
    CGO_ENABLED=1 CC=$COMPILER_NAME GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -a -o teleport-operator github.com/gravitational/teleport/operator

# Create the image with the build operator on the $TARGETPLATFORM
# FROM gcr.io/distroless/cc
FROM --platform=$TARGETPLATFORM gcr.io/distroless/cc
WORKDIR /
COPY --from=builder /workspace/teleport-operator .

ENTRYPOINT ["/teleport-operator"]
