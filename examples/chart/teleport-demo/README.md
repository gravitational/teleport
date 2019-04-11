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
$ ./build-all.sh 3.1.8
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

To install the chart with the release name `teleport` and Teleport version 3.1.8, run:

```bash
$ helm secrets install --name teleport -f secrets/sops/teleport-demo/secrets.yaml ./ --set teleportVersion=3.1.8
```

Once the chart is installed successfully, Helm will output a section titled NOTES containing the URL to access the main
cluster's web UI, along with some example `tsh` commands based on your installation.

You can show these notes again in future with the `helm status <releaseName>` command - e.g. `helm status teleport`

## Deleting the chart

If you named the chart `teleport`:

```bash
$ helm delete --purge teleport
```

Namespaces will automatically be deleted once the cluster is shut down.

## Recreating this without access to secrets

If you're looking to use/modify this code and don't have access to the repo containing the sops-encrypted secrets,
here's the sections you'll need to ensure you have in your `secrets.yaml` or equivalent file:

```yaml
secrets:
  auth0:
    client_id: <Auth0 client ID>
    client_secret: <Auth0 client secret>
  cloudflare:
    api_key: <Cloudflare API key>
    email: <Cloudflare email address>
  license: |
    <PEM-encoded Teleport enterprise license file>
```
