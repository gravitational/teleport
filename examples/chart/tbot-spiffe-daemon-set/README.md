# TBot SPIFFE Daemon Set 

This chart deploys a daemon set of `tbot` agents which are configured to expose
the SPIFFE Workload API via a Unix Domain Socket. This socket can then be
mounted into pods to allow them to receive SPIFFE SVIDs issued by Teleport
Machine & Workload Identity.

## How To

### Basic configuration

Follow steps 1 and 2 from the
[Deploying `tbot` on Kubernetes guide](https://goteleport.com/docs/machine-workload-identity/deployment/kubernetes/)
to create a Bot and Join Token for your `tbot` daemon set to use for
authentication.

The following are the minimal values you must set on the chart for it to 
function correctly:

```yaml
# Set to the name of your Teleport cluster.
clusterName: example.teleport.sh 
# Set to the name of the token you created.
token: example-token
# Set to the address of your Teleport Proxy Service.
teleportProxyAddress: example.teleport.sh:443
workloadIdentitySelector:
  # Set to the name of the WorkloadIdentity resource you'd like to use when 
  # issuing SVIDs.
  name: example-workload-identity 
```

See [values.yaml](./values.yaml) for a full reference of the available values.

## Contributing to the chart

Please read [CONTRIBUTING.md](../CONTRIBUTING.md) before raising a pull request to this chart.