# Build the manager binary
FROM golang:1.18 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# we have to copy the API before `go mod download` because go.mod has a replace directive for it
COPY api/ api/

# cache deps before building and copying source
# this way we don't need to re-download deps when the deps are the same
RUN go mod download

COPY *.go ./
COPY lib/ lib/
COPY operator/apis/ operator/apis/
COPY operator/controllers/ operator/controllers/
COPY operator/sidecar/ operator/sidecar/
COPY operator/main.go operator/main.go
COPY operator/namespace.go operator/namespace.go

# Build
RUN GOOS=linux GOARCH=amd64 go build -a -o teleport-operator github.com/gravitational/teleport/operator

FROM gcr.io/distroless/cc
WORKDIR /
COPY --from=builder /workspace/teleport-operator .

ENTRYPOINT ["/teleport-operator"]
