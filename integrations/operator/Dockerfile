ARG BUILDBOX
ARG BASE_IMAGE=gcr.io/distroless/static-debian11
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

ARG TARGETOS
ARG TARGETARCH

# Build the program. We rely on golang's cross-compilation capabilities for multiarch building.
RUN echo "Targeting $TARGETOS/$TARGETARCH" && \
    GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -a -o /go/bin/teleport-operator github.com/gravitational/teleport/integrations/operator

# Create the image with the build operator on the $TARGETPLATFORM
# TARGETPLATFORM is provided by Docker/buildx
FROM --platform=$TARGETPLATFORM $BASE_IMAGE
WORKDIR /
COPY --from=builder /go/bin/teleport-operator .

ENTRYPOINT ["/teleport-operator"]
