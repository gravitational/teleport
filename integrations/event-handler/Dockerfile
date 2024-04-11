# Build the plugin binary
ARG GO_VERSION
ARG BASE_IMAGE=gcr.io/distroless/static-debian12

FROM golang:${GO_VERSION}-bookworm as builder

ARG GITREF
ARG VERSION

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy the go source
COPY event-handler event-handler

# Build
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build make -C event-handler GITREF=${GITREF} VERSION=${VERSION}

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM $BASE_IMAGE

COPY --from=builder /workspace/event-handler/build/teleport-event-handler /usr/local/bin/teleport-event-handler

ENTRYPOINT ["/usr/local/bin/teleport-event-handler"]
