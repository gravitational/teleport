ARG BUILDBOX_VERSION
FROM public.ecr.aws/gravitational/teleport-buildbox:${BUILDBOX_VERSION} as builder

WORKDIR /go/src/github.com/gravitational/teleport

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
RUN GOOS=linux GOARCH=amd64 go build -a -o build/teleport-operator github.com/gravitational/teleport/operator

FROM gcr.io/distroless/cc
WORKDIR /
COPY --from=builder /go/src/github.com/gravitational/teleport/build/teleport-operator .

ENTRYPOINT ["/teleport-operator"]
