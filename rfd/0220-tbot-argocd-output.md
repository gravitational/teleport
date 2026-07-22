---
authors: Dan Upton (daniel.upton@goteleport.com)
state: draft
---

# RFD 220 - `tbot` Argo CD Output

## Required Approvers

* Engineering: @strideynet && @timothyb89
* Product: @thedevelopnik

## What

Adding a first-class integration with Argo CD to `tbot`, to provide a better
day zero experience.

## Why

Argo CD is a popular continuous delivery tool for Kubernetes. You install it
into a Kubernetes cluster, and then it can deploy applications to either the
same cluster or, more commonly, to other "external" clusters.

Today, if you want to use `tbot` to broker access to external clusters, there is
a lot of manual work involved.

Our current Kubernetes outputs render a kubeconfig file which is compatible with
`kubectl` and many other tools in the ecosystem — in fact, the `argocd` CLI can
even register a cluster based on a context in a kubeconfig file.

There are two forms a `tbot`-generated kubeconfig file can take, both of which
present challenges for use with Argo CD:

1. Where credentials (i.e. certificate and private key) are rendered directly
   into the file
2. Where `tbot` is used as an "exec plugin" to provide credentials on-demand

With the former approach, the credentials will become stale so you'd need to
run `argocd cluster add --upsert` each time tbot renews them.

With the latter approach, you'd need to add the `tbot` binary to your Argo
container images and as `tbot kube credentials` only renders existing
credentials rather than renewing them, you'd also need to mount a volume into
each container.

Multiple users have reported that using `tbot` with Argo CD today is challenging
(e.g. [#41469](https://github.com/gravitational/teleport/discussions/41459)) and
given its prominence in the Kubernetes ecosystem, it's worthwhile having a more
seamless integration.

## Details

Argo CD supports "declaratively" managing clusters
[using Kubernetes secrets](https://argo-cd.readthedocs.io/en/latest/operator-manual/declarative-setup/#clusters),
which are then picked up by Argo's application controller.

Therefore, `tbot` could register clusters by simply rendering the information
its Kubernetes output services already have into secrets in the format Argo
expects.

In our experiments, Argo detected and applied secret changes within a few
seconds. Even if there were a slight delay due to caching, setting the
credential TTL to sufficiently longer than the renewal interval should solve
the issue.

> [!NOTE]
> For the initial version, configurations where the Teleport proxy is behind a
> TLS-terminating load balancer will not be supported, as this would require
> running a local proxy to apply our ALPN upgrade workaround.
>
> We will make this limitation clear in the documentation, and if we detect this
> we will log an explicit error rather than generating unusable configuration.

### Service configuration

We'll add a new service type to `tbot` called `kubernetes/argo-cd`. It will be
based on the `kubernetes/v2` service and support managing multiple Kubernetes
clusters using name and label selectors.

In contrast with the other "output" services, it will not have a `destination`
field — because:

1. The service will only output to Kubernetes secrets
2. The service will write to *many* secrets rather than a single secret like the
   existing `kubernetes_secret` destination does

Instead, you will be able to specify the namespace to which secrets will be
written (defaulting to `$POD_NAMESPACE` or `"default"`), a prefix for the secret
names, as well as any custom labels.

You will also be able to control the Argo cluster's `project`, `namespaces`, and
`cluster_resources` fields.

#### Full example config

```yaml
type: kubernetes/argo-cd
name: my-argo-service
credential_ttl: 1m0s
renewal_interval: 30s
selectors:
  - name: foo
  - labels:
      foo: bar
secret_namespace: argocd
secret_name_prefix: my-argo-cluster-
secret_labels:
  my-label: value
project: super-secret-project
namespaces:
  - prod
  - dev
cluster_resources: true
```

#### Minimal example config

```yaml
type: kubernetes/argo-cd
selectors:
  - name: foo
```

### Secret format

Cluster secrets will be named based on a prefix (defaulting to `teleport.argocd-cluster`)
and a hash of the Teleport and Kubernetes cluster names.

In order for Argo CD to discover them, secrets will be labelled with:

```yaml
argocd.argoproj.io/secret-type: cluster
```

The client certificate, private key, cluster CA certificates, SNI header, and
proxy address will all be rendered inline to avoid the need to add `tbot` to
Argo container images.

#### Example secret

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: teleport.argocd-cluster.a1b2c3d4
  labels:
    argocd.argoproj.io/secret-type: cluster
  annotations:
    teleport.dev/bot-name: argocd-bot
    teleport.dev/kubernetes-cluster-name: interstellar
    teleport.dev/updated: 2025-07-31T13:35:15+00:00
    teleport.dev/tbot-version: v18.2.0
    teleport.dev/teleport-cluster-name: asteroid.earth
type: Opaque
stringData:
  name: asteroid.earth-interstellar
  server: https://asteroid.earth/v1/teleport/foo/bar
  project: my-argo-project
  namespaces: prod,dev
  clusterResources: true
  config: |
    {
      "tlsClientConfig": {
        "insecure": false,
        "caData": "<base64 encoded ca certificates>",
        "certData": "<base64 encoded certificate>",
        "keyData": "<base64 encoded private key>",
        "serverName": "kube-teleport-proxy-alpn.teleport.cluster.local"
      }
    }
```

### Helm chart

It's likely that users are running `tbot` in Kubernetes with the provided Helm
chart, rather than managing the container themselves.

Today, the chart allows you to either provide a full `tbot` configuration file
(`tbotConfig`), a set of outputs/services (`outputs` and `services`) or to
enable the "default" output (`defaultOutput`) that writes a Teleport identity
file to a Kubernetes secret.

We'll add an `argocd` configuration option which enables the new service. For
simplicity, it will be mutually-exclusive with the other options that control
`tbot`'s services (e.g. `defaultOutput`).

#### Example values file

```yaml
clusterName: "asteroid.earth"
teleportProxyAddress: "asteroid.earth:443"
token: "my-join-token"

argocd:
  enabled: true
  clusterSelectors:
    - name: k8s-cluster-1
    - labels:
      environment: production
```
