# WARNING

This chart is **deprecated** and no longer actively maintaned or supported by Teleport.
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

## Prerequisites

- Helm v3
- Kubernetes 1.14+
- A Teleport license file stored as a Kubernetes Secret object - see below

### Prepare the license file

Download the `license.pem` from the Teleport dashboard, and then rename it to the filename that this chart expects:

```console
cp ~/Downloads/license.pem license-enterprise.pem
```

Store it as a Kubernetes secret:

```console
kubectl create secret generic license --from-file=license-enterprise.pem
```

## Installing the chart

Make sure you read `values.yaml` and edit the appropriate sections (particularly the root cluster configuration) before
installing the chart.

To install the chart with the release name `teleport`, run:

```console
helm install teleport ./
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

## Deleting the configured cluster

If you need to delete the chart, you can use:

```console
helm uninstall teleport
```
