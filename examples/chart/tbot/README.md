# TBot Chart

This chart deploys an instance of the Machine ID agent, TBot, into your 
Kubernetes cluster.

To use it, you will need to know:

- The address of your Teleport Proxy or Auth Server
- The name of your Teleport cluster
- The name of a join token configured for Machine ID and your Kubernetes cluster
  (https://goteleport.com/docs/enroll-resources/machine-id/deployment/kubernetes/)

By default, it will write a secret containing an identity for Teleport that can
be used with the Access Plugins, `tctl` or `tsh` tools. You can control the name
of this secret using `defaultOutput.secretName` or can disable this default
output using `defaultOutput.enabled`.

## Contributing to the chart

Please read [CONTRIBUTING.md](../CONTRIBUTING.md) before raising a pull request to this chart.