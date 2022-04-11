# Network

This reserves external static ip addresses to be used
for the [`k8s/proxy`](../k8s/proxy.yaml) and [`k8s/grafana`](../k8s/grafana.yaml) services.
Once you have the ips you **must** setup the DNS records accordingly.

**NOTE**: This only needs to be run once for a GCP project.

## Usage

Confirm that everything is working correctly:

```bash
$ make plan
```

If you are running this automation for the first time, you may be asked to run
`terraform init` before continuing.

```bash
$ make apply
```

---

To release the static ip addresses run:

```bash
$ make destroy
```