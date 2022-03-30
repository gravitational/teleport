# WARNING

This chart is **deprecated** and no longer actively maintained or supported by Teleport.
We recommend the use of the [teleport-cluster](../teleport-cluster/README.md) chart instead.

# Teleport

[Gravitational Teleport](https://github.com/gravitational/teleport) is a modern SSH/Kubernetes API proxy server for
remotely accessing clusters of Linux containers and servers via SSH, HTTPS, or Kubernetes API.

## Introduction

This chart deploys Teleport components to your cluster via a Kubernetes `Deployment`.

By default this chart is configured as follows:

- 1 replica
- Record ssh/k8s exec and attach session to the `emptyDir` of the Teleport pod
  - These sessions will also be stored on the root cluster, if used to access this Helm-configured cluster remotely
- TLS is enabled by default on the Proxy
  - The leaf cluster will generate its own self-signed certificates

See the comments in the default `values.yaml` and also the Teleport documentation for more options.

### Setting up your trusted cluster configuration

This version of the chart has been modified to automatically connect back to a "root" Teleport cluster when started. This
enables remote access to and management of this Teleport cluster (the "leaf" cluster) when deployed on a customer site.

You will need to edit the values in the `trustedCluster.extraVars` section of `values.yaml` as appropriate for your cluster.
There are comments in the file describing what the values need to be set to.

### DaemonSet information

This version of the chart has also been modified to deploy a Kubernetes `DaemonSet` which will run a privileged pod with a large
number of host filesystem mounts on every Kubernetes worker node. Each pod will install and set up Teleport to run on the worker
node itself as a `systemd` service, so that `ssh`-like access to these nodes is possible by logging into the leaf cluster.

The configuration for this is set up in the `daemonset` section of `values.yaml`. You must configure a secure node join token here.

Each node connects back to the auth server of the Teleport leaf cluster running inside Kubernetes by using an exposed `NodePort`.
This setup may not survive failures of the `kubelet` or other underlying node services, so it should **not** be relied on for
emergency 'break-glass' access in the event of a failure.

Note: this chart does **not** work as-is on CoreOS-based distributions (e.g. GKE's Container-Optimized OS) which mount `/usr/local/bin` read-only.

## Prerequisites

- Helm v3
- Kubernetes 1.14+
- A Teleport license file stored as a Kubernetes Secret object - see below
  - This means that by definition, the chart will use an Enterprise version of Teleport

### Prepare the license file

Download the `license.pem` from the [Teleport dashboard](https://dashboard.gravitational.com/web/), and then rename it to the filename that this chart expects:

```console
cp ~/Downloads/license.pem license-enterprise.pem
```

Store it as a Kubernetes secret:

```console
kubectl create secret generic license --from-file=license-enterprise.pem
```

This license will **not** be added to Teleport nodes running via the `DaemonSet`, as a license is only needed when running the auth server role.

## Installing the chart

Make sure you read `values.yaml` and edit the appropriate sections (particularly the root cluster configuration) before
installing the chart. You must also set a secure node join token for use by your Kubernetes worker nodes.

We recommend copying all the parts of `values.yaml` which you need to change to another file called `teleport-values.yaml`,
then providing this file to Helm using the `-f` parameter on the command line. Helm will take the defaults from
`values.yaml` and merge them with your changes from the `teleport-values.yaml` file.

To install the chart with the release name `teleport` and extra values from `teleport-values.yaml`, run:

```console
helm install teleport -f teleport-values.yaml ./
```

You can view debug logs for the cluster with this command:

```console
kubectl logs deploy/teleport -c teleport
```

If you have any issues with the leaf cluster not appearing on the root cluster, look at the logs for the `teleport-sidecar`
container:

```console
kubectl logs deploy/teleport -c teleport-sidecar
```

You can view debug logs for the Teleport service running on the Kubernetes worker nodes with this command:

```console
kubectl logs daemonset/teleport-node
```

If you have multiple worker nodes, look for pods starting with `teleport-node-` in the output of `kubectl get pods` and
use `kubectl logs pod/teleport-node-xxxxxx` to view logs from each node separately.

## Deleting the chart

If you need to delete the chart, you can use:

```console
helm uninstall teleport
```
