# TBot Chart

This chart deploys an instance of the Machine ID agent, TBot, into your 
Kubernetes cluster.

To use it, you will need to know:

- The address of your Teleport Proxy Service or Auth Service
- The name of your Teleport cluster
- The name of a join token configured for Machine ID and your Kubernetes cluster
  (https://goteleport.com/docs/enroll-resources/machine-id/deployment/kubernetes/)

By default, this chart is designed to use the `kubernetes` join method but it
can be customised to use any delegated join method. We do not recommend that
you use the `token` join method with this chart.

## How To

### Basic configuration

This basic configuration will write a Teleport identity file to a secret in
the deployment namespace called `test-output`.

```yaml
clusterName: "test.teleport.sh"
teleportProxyAddress: "test.teleport.sh:443"
defaultOutput:
  secretName: "test-output"
token: "my-token"
```

### Customization

When customizing the configuration of `tbot` using this chart, you can either
choose to leverage the more granular values or use the `tbotConfig` value
to inject custom configuration into any field. We recommend using the granular
values where possible to better benefit from changes we may introduce in future
updates.

## Contributing to the chart

Please read [CONTRIBUTING.md](../CONTRIBUTING.md) before raising a pull request to this chart.