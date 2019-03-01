# Teleport on Kubernetes

[Gravitational Teleport](https://github.com/gravitational/teleport) is a modern SSH/Kubernetes API proxy server for
remotely accessing clusters of Linux containers and servers via SSH, HTTPS, or Kubernetes API.

This configuration is quite a Gravitational-specific deployment but should show a good amount of reusability for other
savvy admins.

## Introduction

This chart deploys Teleport components to your cluster using various Kubernetes primitives.

It supports a few key features:
- A configurable number of nodes per cluster (n)
- One 'main' cluster with <n> nodes in its own Kubernetes namespace
- Any amount of different-named trusted clusters with <n> nodes, each in their own Kubernetes namespace
    - These clusters are automatically linked to 'main' as trusted clusters
- OIDC authentication via Auth0
- DNS records pointing to a Kubernetes LoadBalancer for each cluster, set up on a configurable Cloudflare domain
- LetsEncrypt certificates automatically provisioned, configured and renewed for each cluster via certbot-dns-cloudflare
- Secrets encrypted using sops and a key from GKE

See the comments in the default `values.yaml` and also the [Teleport documentation](https://gravitational.com/teleport/docs/quickstart) for more options.

## Prerequisites

- Kubernetes 1.10+
- [sops](https://github.com/mozilla/sops)
- [helm-secrets](https://github.com/futuresimple/helm-secrets)
- [gcloud SDK](https://cloud.google.com/sdk/docs/downloads-interactive)
    - ```curl https://sdk.cloud.google.com | bash``` for a simple install
- Secrets stored in secrets.yaml and encrypted with sops
    - Teleport Enterprise license
    - Email address and API key for a Cloudflare account that controls the domain you wish to use
    - Client ID and client secret for a configured Auth0 application

## Installing the chart

If you want to use a different version of Teleport, you should build and push the Docker images for the specified
version to GCR:

```
$ cd examples/chart/teleport-demo/docker
$ gcloud auth login
$ gcloud auth configure-docker
$ ./build-all.sh 3.1.7
```

Make sure that you have access to the key for sops encryption:
```bash
$ gcloud auth application-default login
$ gcloud kms keys list --location global --keyring teleport-sops
NAME                                                                                          PURPOSE          LABELS  PRIMARY_ID  PRIMARY_STATE
projects/kubeadm-167321/locations/global/keyRings/teleport-sops/cryptoKeys/teleport-sops-key  ENCRYPT_DECRYPT          1           ENABLED
```

kubectl needs to know about your cluster - for GKE you can use something like this:

```bash
$ gcloud container clusters get-credentials <cluster-name> --zone <zone> --project <project>
$ ./gke-init.sh
```

Make sure that you have updated the submodule containing the secrets:

```bash
$ git pull --recurse-submodules
```

To install the chart with the release name `teleport` and Teleport version 3.1.7, run:

```bash
$ helm secrets install --name teleport -f secrets/sops/teleport-demo/secrets.yaml ./ --set teleportVersion=3.1.7
```

Once the chart is installed successfully, you should be able to go to https://[mainClusterName].[cloudflareDomain]:3080 and log in with
Auth0 - i.e. https://main.gravitational.co:3080

## Deleting the chart

If you named the chart `teleport`:

```bash
$ helm delete --purge teleport
```

Namespaces will automatically be deleted once the cluster is shut down.