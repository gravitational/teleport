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
  enabled: yes
  listen_addr: localhost:3081
  mtls: yes
  keypairs:
  - key_file: key.pem
    cert_file: cert.pem
  ca_certs:
  - ca.pem
```

This implementation will only support user provided certs and CA for now.
Using Teleport's Host CA and generated certs is an option that can be considered in the future for self hosted teleport instances. That's not optimal for teleport cloud because prometheus would have to wait for teleport to start before it could be provisioned.
There are other security and design concerns you can read about here https://github.com/gravitational/teleport/pull/6469

### Backwards compatibility

The existance of the new metrics service and the service currently hosted at the `diag-addr` will be mutually exclusive.
Having the metrics service enabled in the config will stop metrics from being hosted at the old endpoint but it will still be available for the forseable future for those who wish to continue using it.
There is no current timeline on when it will be deprecated.

## Migration

To use the new metrics service, prometheus will have to be reconfigured to start listening at the new address defined in the config alongside using certs for mtls if needed.


### Summary

As a summary here is a self explanatory config example
```
metrics_service:
  # 'enabled: no' or the absence of this section alltogether means that metrics
  # will still be hosted at the 'diag-addr' provided to teleport start as a flag..
  enabled: yes
  # 'listen_addr' is the new address where the metrics will be hosted
  listen_addr: localhost:3081
  # 'mtls: no' will ship metrics in clear text to prometheus
  mtls: yes
  # 'keypairs' should be provided alongside 'mtls: yes'. Only user generated
  # certs and ca are currently supported, but that can change to support
  # certs provided by teleport if there is a demand for it
  keypairs:
  - key_file: key.pem
    cert_file: cert.pem
  # 'ca_certs' should be provided alongside 'mtls: yes'. Those are the CA certs
  # of the prometheus instances consuming the metrics
  ca_certs:
  - ca.pem
```


### Additional work

Update the documentation with all the relevant changes
