ARG GO_VERSION=1.26.7
FROM golang:${GO_VERSION} AS builder

ENV DEBIAN_FRONTEND=noninteractive

RUN apt-get update && apt-get install --no-install-recommends -y \
    build-essential \
    libpam-dev \
    && rm -rf /var/lib/apt/lists/*

ENV GOMODCACHE=/go/pkg/mod

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go install github.com/go-delve/delve/cmd/dlv@v1.26.3

WORKDIR /src

COPY go.mod go.sum ./
COPY api/go.mod api/go.sum ./api/
COPY e/tests/antithesis/workloads/go.mod e/tests/antithesis/workloads/go.sum ./e/tests/antithesis/workloads/

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

RUN --mount=type=cache,target=/go/pkg/mod \
    cd e/tests/antithesis/workloads/ && \
    go mod download


# TODO(okraport): There may be better ways to optimize this, unfortunately the Antithesis instrumentation requires
# the entire source tree to be regenerated before building, which also means copying the entire source tree into a layer
# which relies heavily on teleport/Dockerfile.dockerignore to avoid files that easily invalidate the docker cache.

# Copy sources, ignored paths are listed in Dockerfile.dockerignore
COPY . ./
# Patch out the webassets, the GUI is not usable within Antithesis and skipping the webassets
# saves some time.
COPY e/tests/antithesis/docker/teleport/webassets_override.go webassets_noembed.go

RUN --mount=type=cache,target=/go/pkg/mod \
    go get github.com/antithesishq/antithesis-sdk-go@v0.7.2 && \
    go get -tool github.com/antithesishq/antithesis-sdk-go/tools/antithesis-go-instrumentor@v0.7.2

# Instrument the source tree. This creates customer, notifier, and symbols
# under the empty output directory.
# https://antithesis.com/docs/using_antithesis/sdk/go/instrumentor/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    mkdir /instrumented && \
    go tool antithesis-go-instrumentor \
        -skip_test_files=true \
        -skip_protobuf_files=true \
        . /instrumented && \
    test -f /instrumented/customer/go.mod

WORKDIR /instrumented/customer

ARG TELEPORT_MODULES_BUILD_TYPE=ent
# Teleport binaries built from instrumented sources.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -p 8 -race -tags antithesis \
    -ldflags "-X github.com/gravitational/teleport/lib/modules.teleportBuildType=${TELEPORT_MODULES_BUILD_TYPE}" \
    -o /out/teleport ./tool/teleport
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -p 8 -race -tags antithesis \
    -ldflags "-X github.com/gravitational/teleport/lib/modules.teleportBuildType=${TELEPORT_MODULES_BUILD_TYPE}" \
    -o /out/tsh ./tool/tsh
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -p 8 -race -tags antithesis \
    -ldflags "-X github.com/gravitational/teleport/lib/modules.teleportBuildType=${TELEPORT_MODULES_BUILD_TYPE}" \
    -o /out/tctl ./tool/tctl
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -p 8 -race -tags antithesis \
    -ldflags "-X github.com/gravitational/teleport/lib/modules.teleportBuildType=${TELEPORT_MODULES_BUILD_TYPE}" \
    -o /out/tbot ./tool/tbot

WORKDIR /instrumented/customer/e/tests/antithesis/workloads

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -p 8 -race -tags antithesis \
        -o /out/workload_entrypoint ./cmd/workload_entrypoint

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -p 8 -race -tags antithesis \
        -o /out/eventually_cache_is_healthy ./cmd/eventually_cache_is_healthy

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -p 8 -race -tags antithesis \
        -o /out/eventually_auth_consistent ./cmd/eventually_auth_consistent

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -p 8 -race -tags antithesis \
        -o /out/parallel_driver_crud_fuzzer ./cmd/parallel_driver_crud_fuzzer

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -p 8 -race -tags antithesis \
        -o /out/parallel_driver_single_ssh_command ./cmd/parallel_driver_single_ssh_command

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=1 go build -p 8 -race -tags antithesis \
        -o /out/parallel_driver_app_access ./cmd/parallel_driver_app_access

FROM debian:bookworm-slim AS runtime

ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && apt-get install -y \
    ca-certificates \
    bash \
    dumb-init \
    curl \
    libpam0g \
    && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /var/lib/teleport /etc/teleport

COPY --from=builder /out/teleport           /usr/local/bin/teleport
COPY --from=builder /out/tsh                /usr/local/bin/tsh
COPY --from=builder /out/tctl               /usr/local/bin/tctl
COPY --from=builder /out/tbot               /usr/local/bin/tbot
COPY --from=builder /instrumented/symbols/  /symbols/
COPY --from=builder /go/bin/dlv             /usr/local/bin/dlv

ENV NO_COLOR=1
# Attempt a graceful shutdown by default
STOPSIGNAL SIGQUIT

# TODO(okraport): It is a known limitation that when node termination is enabled on Antithesis it is possible the fuzzer
# sends a SIGTERM during go init before the signal handler is installed. This results in the runtime exiting with exit code 2
# as that is the default for the runtime. Antithesis expects a known code instead such as 137 and 143, which correspond to
# SIGKILL and SIGTERM on the container process and as such flags this as a finding. One possible option is to write a thin wrapper
# similiar to dumb-init which can remap the signals until the debug endpoint is registered to signal that the handlers are likely in
# place. Decide whether this will be handled by a wrapper or by Antithesis changing how the timing on node termination fault injection
# is structured.
COPY e/tests/antithesis/docker/teleport/entrypoint.sh /usr/local/bin/entrypoint.sh
ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

FROM runtime AS teleport
# Every teleport cache instance should eventually recover. Note that this command is placed in every test template,
# this allows the fuzzer a possiblity to find a series of faults and/or commands that could trip the cache into an
# unhealthy state. This verifies the liveness property of Teleport cache to recover from failure.
COPY --from=builder /out/eventually_cache_is_healthy /opt/antithesis/test/v1/crud/eventually_cache_is_healthy
COPY --from=builder /out/eventually_cache_is_healthy /opt/antithesis/test/v1/ssh/eventually_cache_is_healthy
COPY --from=builder /out/eventually_cache_is_healthy /opt/antithesis/test/v1/apps/eventually_cache_is_healthy

CMD ["/usr/local/bin/teleport", "start"]


FROM teleport AS auth
# Every teleport auth instance should be eventually consistent with the backend. Note that this command is placed in every test template,
# this allows the fuzzer a possiblity to find a series of faults and/or commands that could trip the cache into an
# inconsistent view of the backend. This verifies the liveness property of Teleport auth to be eventually consistent with the
# backend.
COPY --from=builder /out/eventually_auth_consistent /opt/antithesis/test/v1/crud/eventually_auth_consistent
COPY --from=builder /out/eventually_auth_consistent /opt/antithesis/test/v1/ssh/eventually_auth_consistent
COPY --from=builder /out/eventually_auth_consistent /opt/antithesis/test/v1/apps/eventually_auth_consistent

CMD ["/usr/local/bin/teleport", "start", "--bootstrap=/etc/teleport/bootstrap.yaml", "--apply-on-startup=/etc/teleport/startup.yaml"]

FROM runtime AS tbot
CMD ["/usr/local/bin/tbot","start", "--config=/etc/tbot/admin.yaml"]


FROM runtime AS workload

RUN mkdir -p \
    /etc/tbot \
    /opt/machine-id \
    /var/lib/teleport/bot

COPY --from=builder /out/workload_entrypoint /usr/local/bin/workload_entrypoint
CMD ["/usr/local/bin/workload_entrypoint", "--config=/etc/tbot/admin.yaml", "--config=/etc/tbot/allowed.yaml", "--config=/etc/tbot/denied.yaml", "--config=/etc/tbot/crud-denied.yaml"]

# See https://antithesis.com/docs/test_templates/test_composer_reference/
FROM workload AS workload-core
COPY --from=builder /out/parallel_driver_crud_fuzzer /opt/antithesis/test/v1/crud/parallel_driver_crud_fuzzer
COPY --from=builder /out/parallel_driver_single_ssh_command /opt/antithesis/test/v1/ssh/parallel_driver_single_ssh_command
COPY --from=builder /out/parallel_driver_app_access /opt/antithesis/test/v1/apps/parallel_driver_app_access

