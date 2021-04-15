---
author: Naji Obeid (naji@goteleport.com)
state: draft
---

# RFD 27 - mTLS Prometheus metrics

## What

This RFD proposes the option of securing the `/metrics` endpoint read by prometheus with mTLS.

## Why

To prevent leaking metrics shipped to prometheus over an insecure network.

## Details

Currently the `/metrics` handler resides under `initDiagnosticService()` in `lib/service/service.go` which will be hosted at the address provided by the `--diag-addr` flag given to `teleport start`.

We could have enabled TLS globally in the server implemented in `initDiagnosticService` and only verify it in the metrics handler while keeping the other healthz readyz and debug endpoints intact. That would trigger a tls renegotiation which is both [not supported by prometheus](https://github.com/prometheus/prometheus/issues/1998) and not supported in TLS1.3 anymore. So this isn't really an option we can consider.
So in order to achieve this we have to move the `/metrics` endpoint to its own `initMetricsService` server where tls is either on/off depending on the settings supplied in the config.

The config would ideally be something like:
```
metrics_service:
  enabled: "yes"
  listen_addr: ...
  keypairs:
    - key_file: /var/lib/teleport/...
      cert_file: /var/lib/teleport/...1
  prometheus_ca_cert: /var/lib/teleport/...
```

### Backwards compatibility

This is a breaking change for systems relying on metrics being hosted at the `diag-addr`. People would have to update their configurations and use the new endpoint.
Unless we keep supporting the old endpoint for insecure metrics delivery and disable it if mTLS is configured to be used through the new one, which is not a clean solution in my opinion.

### Additional work

Update the documentation with all the relevant changes
