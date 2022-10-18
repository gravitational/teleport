ARG BUILDBOX
FROM $BUILDBOX as builder

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
COPY operator/apis/ operator/apis/
COPY operator/controllers/ operator/controllers/
COPY operator/sidecar/ operator/sidecar/
COPY operator/main.go operator/main.go
COPY operator/namespace.go operator/namespace.go

# Build
RUN GOOS=linux GOARCH=amd64 go build -a -o /go/bin/teleport-operator github.com/gravitational/teleport/operator

# Create the image with the build operator on the $TARGETPLATFORM
# FROM gcr.io/distroless/cc
FROM --platform=$TARGETPLATFORM gcr.io/distroless/cc
WORKDIR /
COPY --from=builder /go/bin/teleport-operator .

ENTRYPOINT ["/teleport-operator"]
