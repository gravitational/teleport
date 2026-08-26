---
author: Naji Obeid (naji@goteleport.com)
state: implemented
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

This implementation will only support user provided certs and CA for now.
Using Teleport's Host CA and generated certs is an option that can be considered in the future for self hosted teleport instances. That's not optimal for teleport cloud because prometheus would have to wait for teleport to start before it could be provisioned.
There are other security and design concerns you can read about here https://github.com/gravitational/teleport/pull/6469

The metrics's service config will look like the following.n
```
metrics_service:
  # 'enabled: no' or the absence of this section alltogether means that metrics
  # will still be hosted at the 'diag-addr' provided to teleport start as a flag.
  enabled: yes
  # 'listen_addr' is the new address where the metrics will be hosted.
  # defaults to port 3081
  listen_addr: localhost:3081
  # 'mtls: no' will ship metrics in clear text to prometheus.
  mtls: yes
  # 'keypairs' should be provided alongside 'mtls: yes'. Only user generated
  # certs and ca are currently supported, but that can change to support
  # certs provided by teleport if there is a demand for it.
  keypairs:
  - key_file: key.pem
    cert_file: cert.pem
  # 'ca_certs' should be provided alongside 'mtls: yes'. Those are the CA certs
  # of the prometheus instances consuming the metrics.
  ca_certs:
  - ca.pem
```


### Backwards compatibility

Having the `metrics_service` enabled in the config will override metrics from being hosted at the `diag-addr` endpoint.
To be clear metrics will be only available through the metrics service if both the `metrics_service` is enabled and the `diag-addr` is set.
Teleport will still support shipping metrics over the `diag-addr` endpoint for those who wish to continue using it and there is no current timeline on when it will be deprecated.


## Migration

To use the new metrics service, prometheus will have to be reconfigured to start listening at the new address defined in the config alongside using certs for mtls if needed.

Here are the steps to a simple migration scenario:
- Add a new job to the prometheus config that aims to start listening on the new `metrics_service` endpoint. mTLS is optional. Example:
```
- job_name: 'teleport_new_metrics_service'
  scheme: https
  tls_config:
    ca_file: "ca.pem"
    cert_file: "cert.pem"
    key_file: "key.pem"
  metrics_path: /metrics
  static_configs:
   - targets:
     - localhost:3081
```
- Reload or restart prometheus
- Update the teleport config to add the new `metrics_service`. Example:
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
- Restart teleport.

### Notes:
- Modifying the teleport config before having added the new prometheus job will cause a gap in the metrics between the time the metrics service is up and the prometheus config is updated to pull from that address. That's mainly because metrics will not be available to prometheus over the old endpoint at `diag-addr` when `metrics_service` is up.
- Modifying the existing prometheus job that pulls from `diag-addr` to pull from the `metrics_service` before updating Teleport with the new config will cause a gap in the metrics between the time prometheus has been updated and the new teleport config has been applied.


### Additional work

Update the documentation with all the relevant changes
