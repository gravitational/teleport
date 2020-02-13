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

First, check whether there's already a tag for the version of Teleport you want to use in GCR:

```bash
$ gcloud auth login
$ gcloud container images list-tags gcr.io/kubeadm-167321/teleport-ent --filter="tags:4.2.2" # replace 4.2.2 with the Teleport version you want
DIGEST        TAGS                   TIMESTAMP
e2ff7a110d2c  4.2.2                  2020-02-13T16:59:29
```

You can also list all avaliable tags with `gcloud container images list-tags gcr.io/kubeadm-167321/teleport-ent`.

If there isn't already a tag for the version of Teleport you're looking to use, you can build and push the Docker images for the specified version to GCR:

```bash
$ cd examples/chart/teleport-demo/docker
$ gcloud auth configure-docker
$ ./build-all.sh 4.2.2 # replace 4.2.2 with the Teleport version you want to build and push
```

Make sure that you have access to the key for sops encryption:

```bash
$ gcloud auth application-default login
$ gcloud config set project kubeadm-167321
$ gcloud kms keys list --location global --keyring teleport-sops
NAME                                                                                          PURPOSE          LABELS  PRIMARY_ID  PRIMARY_STATE
projects/kubeadm-167321/locations/global/keyRings/teleport-sops/cryptoKeys/teleport-sops-key  ENCRYPT_DECRYPT          1           ENABLED
```

kubectl needs to know about your cluster - for GKE you can use something like this:

```bash
$ gcloud container clusters get-credentials <cluster-name> --zone <zone> --project <project>
$ ./gke-init.sh
```

Make sure that you have updated the submodule containing the secrets. When prompted to authenticate, use a 
personal access token rather than a password:

```bash
$ git pull --recurse-submodules
```

To install the chart with the release name `teleportdemo` and Teleport version 4.2.2, run:

```bash
$ helm secrets install --name teleportdemo -f secrets/sops/teleport-demo/secrets.yaml ./ --set teleportVersion=4.2.2
```

Once the chart is installed successfully, Helm will output a section titled NOTES containing the URL to access the main
cluster's web UI, along with some example `tsh` commands based on your installation.

You can show these notes again in future with the `helm status <releaseName>` command - e.g. `helm status teleportdemo`

## Deleting the chart

If you named the chart `teleportdemo`:

```bash
$ helm delete --purge teleportdemo
```

Namespaces will automatically be deleted once the cluster is shut down. If a deployment fails for some reason and you find you can't delete it with the command above, try skipping the post-delete hooks like this:

```bash
$ helm delete --purge --no-hooks teleportdemo
```

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
