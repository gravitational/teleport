ARG BUILDBOX
ARG BASE_IMAGE

FROM $BUILDBOX-$TARGETARCH as builder

WORKDIR /workspace

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# We have to copy the API before `go mod download` because go.mod has a replace directive for it
COPY api/ api/

RUN go mod download

# Copy in code
COPY tool/tbot tool/tbot
COPY lib/ lib/
COPY api/ api/
COPY gen/ gen/
COPY build.assets/ build.assets/
COPY entitlements/ entitlements/
COPY *.go ./
COPY Makefile Makefile
COPY darwin-signing.mk darwin-signing.mk
COPY common.mk common.mk

RUN make build/tbot

FROM $BASE_IMAGE
# https://github.com/spacelift-io/runner-terraform/blob/main/Dockerfile

# Switch to root so we can invoke chmod
USER root

# Copy in `tbot`
COPY --from=builder /workspace/build/tbot /usr/local/bin
RUN chmod 755 /usr/local/bin/tbot

# Switch back to the default spacelift user.
USER spacelift
