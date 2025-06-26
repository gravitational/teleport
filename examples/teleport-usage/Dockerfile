ARG BASE_IMAGE=gcr.io/distroless/static-debian12

FROM golang:1.22-bookworm@sha256:3d699e4d15d0f8f13c9195c0632a16702b8cbdece2955af1c23b37ae5d55a253 as builder

WORKDIR /go/src/github.com/gravitational/teleport/examples/teleport-usage

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Download and Cache dependencies before building and copying source
# This will prevent re-downloading dependencies if they have not changed, as this
# `run` layer will be cached
RUN go mod download

COPY main.go main.go

RUN CGO_ENABLED=0 go build -o /go/bin/teleport_usage

FROM $BASE_IMAGE
WORKDIR /
COPY --from=builder /go/bin/teleport_usage .

ENTRYPOINT ["/teleport_usage"]
