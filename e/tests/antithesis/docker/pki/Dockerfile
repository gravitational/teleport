# syntax=docker/dockerfile:1
ARG GO_VERSION=1.26.7
FROM --platform=$BUILDPLATFORM golang:${GO_VERSION} AS license-builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /src

COPY go.mod go.sum ./
COPY api/go.mod api/go.sum ./api/
COPY e/tests/antithesis/workloads/go.mod e/tests/antithesis/workloads/go.sum ./e/tests/antithesis/workloads/
RUN --mount=type=cache,target=/go/pkg/mod \
    cd e/tests/antithesis/workloads && \
    go mod download

COPY api ./api

WORKDIR /src/e/tests/antithesis/workloads

COPY e/tests/antithesis/workloads/cmd/helper_generate_license ./cmd/helper_generate_license
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    target_os="${TARGETOS:-linux}"; \
    target_arch="${TARGETARCH:-$(go env GOARCH)}"; \
    CGO_ENABLED=0 GOOS="${target_os}" GOARCH="${target_arch}" \
        go build -o /out/helper_generate_license ./cmd/helper_generate_license

FROM alpine:3.24@sha256:28bd5fe8b56d1bd048e5babf5b10710ebe0bae67db86916198a6eec434943f8b
ARG TARGETARCH
ARG MKCERT_VERSION=1.4.4

RUN apk add --no-cache \
    bash \
    ca-certificates \
    nss-tools \
    openssl \
    wget \
    && wget -O /usr/local/bin/mkcert \
       "https://github.com/FiloSottile/mkcert/releases/download/v${MKCERT_VERSION}/mkcert-v${MKCERT_VERSION}-linux-${TARGETARCH}" \
    && chmod +x /usr/local/bin/mkcert

COPY --from=license-builder /out/helper_generate_license /usr/local/bin/helper_generate_license
COPY e/tests/antithesis/docker/pki/setup.sh /setup.sh
RUN chmod +x /setup.sh

ENTRYPOINT ["/setup.sh"]
