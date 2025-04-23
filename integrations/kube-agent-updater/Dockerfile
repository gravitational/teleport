ARG BUILDBOX
ARG BASE_IMAGE=gcr.io/distroless/static-debian12

# BUILDPLATFORM is provided by Docker/buildx
FROM --platform=$BUILDPLATFORM $BUILDBOX as builder

WORKDIR /go/src/github.com/gravitational/teleport/integrations/kube-agent-updater

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# We have to copy the API before `go mod download` because go.mod has a replace directive for it
COPY api/ api/

RUN go mod download

# Copy in code
COPY lib/ lib/
COPY *.go ./
COPY integrations/kube-agent-updater/ integrations/kube-agent-updater/

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
