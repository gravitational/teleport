---
authors: Andrew Lytvynov (andrew@goteleport.com)
state: implemented
---

# RFD 5 - Kubernetes (k8s) service enhancements

## What

A series of enhancements to the k8s integration:
- k8s service separated from Proxy service
- support for multiple k8s clusters per Teleport cluster
- support for k8s clusters with blocked ingress (without Trusted
  Clusters)
- k8s cluster info available via Teleport API
- automatic login prompt in `kubectl` on session expiry
- audit logging of all k8s API requests
- RBAC for k8s cluster access

## Why

k8s integration in Teleport has been implemented as an optional addon to
the Proxy service. It has several limitations:

- only one k8s cluster per Teleport cluster supported
  - multiple clusters are supported via Trusted Clusters, which is more
    operationally complex
  - a k8s cluster behind network obstacles (firewalls/NATs/etc) also
    requires a Trusted Cluster
- doesn't follow the "separation of concerns" principle in the proxy
- expired user certificate leads to obscure TLS errors from `kubectl`
- no easy way to tell whether a Teleport cluster has a k8s cluster
  behind it
- audit log ignores all non-interactive k8s requests

## Details

### Backwards compatibility

The old k8s integration remains supported and unchanged. Users who don't
migrate to the new integration will not be broken.

The new k8s service will be preferred to the old Proxy-based integration
and all new features will be added to the new service only.

### Mental model of services

The following mental model of Teleport services is used to guide the rest of
this design:
- `auth_service` - command center of the Teleport cluster; handles state,
  credential provisioning and authorization
- `proxy_service` - stateless router (aka bastion, aka gateway) for incoming
  external connections; this is the only Teleport service that needs to be
  exposed publicly to users
- `ssh_service`/`application_service`/`kubernetes_service` - stateless node
  representing a single\* host/web app/k8s cluster; these
  register with the `auth_service` (directly or via a reverse tunnel through
  the `proxy_service`)
  - these services only handle connections coming via `proxy_service`
  - \* `application_service` and `kubernetes_service` can represent multiple
    apps/k8s clusters, as a resource optimization; regardless, they are the
    final hop before client destination

### Service definition

To separate k8s integration from Proxy (and all associated semantics,
like the discovery protocol), a new `kubernetes_service` will be added in
`teleport.yaml` and can be enabled independently from `proxy_service`. This is
similar to `ssh_service` and the upcoming `application_service`.

The new k8s service will inherit all of the configuration and behavior
of the existing `kubernetes` section of `proxy_service`:

```yaml
# Old format:
proxy_service:
    kubernetes:
        enabled: yes
        public_addr: [k8s.example.com:3026]
        listen_addr: 0.0.0.0:3026
        kubeconfig_file: /secrets/kubeconfig
```

but will exist in a new top-level config:

```yaml
# New format:
kubernetes_service:
    enabled: yes
    public_addr: [k8s.example.com:3026]
    listen_addr: 0.0.0.0:3026
    kubeconfig_file: /secrets/kubeconfig
```

In addition to keeping the existing fields, `kubernetes_service` adds several
new ones described below:
- `kube_cluster_name`
- `labels`

Fields `public_addr` and `listen_addr` become optional. Without them, the
service connects to the cluster via [a reverse tunnel through a
proxy](#service-reverse-tunnels).

The k8s service implements all the common service features:
- connecting to an Auth server directly or via a Proxy tunnel
- registration using join tokens (with a dedicated role)
- heartbeats to announce presence
- audit logging

#### Public endpoint

The k8s service does not serve client requests directly. Clients must connect
through a `proxy_service`. To expose a k8s listening port on the proxy, users
must set `kube_listen_addr` config option:

```yaml
proxy_service:
  enabled: yes
  public_addr: example.com
  kube_listen_addr: 0.0.0.0:3026
```

Note: this is equivalent to the old format:

```yaml
proxy_service:
  enabled: yes
  public_addr: example.com
  kubernetes:
    enabled: yes
    listen_addr: 0.0.0.0:3026
```

### Multi-cluster support

Multiple k8s clusters are supported per Teleport clusters. They can be
connected in several different ways.

#### Service inside k8s

This will be the recommended way to use Teleport in k8s.

Service inside k8s does not have a kubeconfig. Instead, it uses the pod service
account credentials to authenticate to its k8s API.

The name of the k8s cluster is specified in the
`kubernetes_service.kube_cluster_name` field of `teleport.yaml`. There is no
portable way to guess the k8s cluster name from the environment.

#### Service outside of k8s

A `kubeconfig_file` provided in `kubernetes_service` (or
`proxy_service.kubeconfig`). All k8s API contexts from it are parsed and
registered.

For example:

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://a.example.com
  name: a-cluster
- cluster:
    server: https://b.example.com
  name: b-cluster
contexts:
- context:
    cluster: a-cluster
    user: a-creds
  name: a
- context:
    cluster: b-cluster
    user: b-creds
  name: b
current-context: a
users:
- name: a-creds
  user:
    client-key-data: "..."
    client-certificate-data: "..."
- name: b-creds
  user:
    client-key-data: "..."
    client-certificate-data: "..."
```

Here, we have two k8s contexts. A proxy will refer to them as “a” and “b” from
the `contexts` section, not “a-cluster” and “b-cluster” from the `clusters`
section. An item in the `clusters` section doesn't specify which credentials to
use on its own and there can be multiple credentials bound to the same cluster.

### K8s cluster metadata

With this, we need to make the clients aware of all the available k8s clusters.
To do this, registered k8s services announce the k8s clusters via heartbeats.
The `ServerSpecV2` gets a new field `kube_clusters` with a list of cluster
names.

Auth server and proxies can be queried for a union of all the k8s clusters
announced by all the proxies.

`tsh` uses this query to show all available k8s clusters to the user (`tsh kube
clusters`).

### Routing

Now that the client knows about the available k8s clusters, they should be able
to switch between them. Since we don’t control the actual k8s client
(`kubectl`), our only transport to indicate the target k8s cluster is the user
TLS certificate.

The TLS certificates we issue for k8s access will include the k8s cluster name
as an extension to the `Subject`, similar to how `kube_users` and `kube_groups`
are embedded.

This is similar to how the `RouteToCluster` field works today for routing to
trusted clusters.

When a proxy receives a k8s request, it does the following:
- Authenticate the request (verify cert)
- Parse target k8s cluster name from the cert extension
- Check it against its local `kubernetes_service`
- If found, forward the request and return
- If not found, fetch the list of all k8s clusters from auth server
- If k8s cluster is found in that list, forward the request to an appropriate address
  - The target address could be a `kubernetes_service`
  - Or another proxy that has a tunnel to the `kubernetes_service`
- If k8s cluster is not found in that mapping, return an error

### Service reverse tunnels

Next, we need to support k8s clusters behind firewalls.

Reusing the reverse tunnel logic from trusted clusters and IoT mode for nodes,
we allow k8s service within the same cluster to tunnel to proxies. From the
user perspective, it’s identical to IoT node setup: use a public proxy address
in the `auth_servers` field of `teleport.yaml`.

When connecting over a reverse tunnel, `kubernetes_service` will not listen on
any local port, unless its `listen_addr` is set.

### CLI changes

#### "tsh kube" commands

`tsh kube` commands are used to query registered clusters and switch
`kubeconfig` context:

```sh
$ tsh login --proxy=proxy.example.com --user=awly
...

# list all registered clusters
$ tsh kube ls
Cluster Name       Status
-------------      ------
a.k8s.example.com  online
b.k8s.example.com  online
c.k8s.example.com  online

# on login, kubeconfig is pointed at the first cluster (alphabetically)
$ kubectl config current-context
awly@a.k8s.example.com

# but all clusters are populated as contexts
$ kubectl config get-contexts
CURRENT   NAME                     CLUSTER             AUTHINFO
*         awly@a.k8s.example.com   proxy.example.com   awly@a.k8s.example.com
          awly@b.k8s.example.com   proxy.example.com   awly@b.k8s.example.com
          awly@c.k8s.example.com   proxy.example.com   awly@c.k8s.example.com

# switch between different clusters:
$ tsh kube login c.k8s.example.com
# Or
$ kubectl config use-context awly@c.k8s.example.com

# check current cluster
$ kubectl config current-context
awly@c.k8s.example.com
```

#### Kubectl exec plugins

[Exec authn
plugins](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#client-go-credential-plugins)
for kubectl are a way to dynamically provision credentials (either bearer token
or client key/cert) for `kubectl` instead of storing them statically in
kubeconfig.

`tsh` will implement the exec plugin spec when executed as `tsh kube
credentials --kube-cluster=foo` (where `foo` is a registered k8s cluster).
`tsh` will cache the certificate on disk before returning.

If client cert is expired or missing, `tsh` falls back to the login prompt.

### RBAC

Since auth servers are now aware of all k8s clusters, we can allow admins to
restrict which k8s clusters a user has permission to access. This allows an
organization to maintain a single Teleport cluster for all k8s access while
following the principle of least privilege.

Access control is based on labels, just like with nodes. Labels are specified
in the teleport.yaml:

```yaml
teleport:
  auth_servers: ["auth.example.com"]
kubernetes_service:
  enabled: yes
  kube_cluster_name: "a.k8s.example.com"
  labels:
    env: prod
  commands:
  - name: kube_service_hostname
    command: [hostname]
    period: 1h
```

and restricted via roles:

```yaml
kind: role
version: v3
metadata:
  name: dev
spec:
  allow:
    kubernetes_groups:
    - 'system:masters'
    kubernetes_labels:
    - env: prod
    - region: us-west1
    - cluster_name: '^us.*\.example\.com$' # cluster_name label is generated automatically
```

### Audit log

The existing k8s integration records all interactive sessions (`kubectl
exec/port-forward`) in the audit log.

The new Kubernetes service will record *all* k8s API requests. This also
applies when using k8s via `proxy_service.kubernetes`.

### Config examples

#### Scenario 1

Proxy running inside a k8s cluster.

`teleport-root.yaml`:
```yaml
auth_service:
  cluster_name: example.com
  public_addr: auth.example.com:3025

proxy_service:
  public_addr: proxy.example.com:3080
  kube_listen_addr: 0.0.0.0:3026

kubernetes_service:
  enabled: yes
```

```sh
$ tsh kube ls
Cluster Name       Status
-------------      ------
example.com        online
```

Note: `kubernetes_service` doesn't explicitly set `kube_cluster_name`, so it
defaults to the Teleport `cluster_name`.

Next, user connects a new k8s cluster to the existing auth server:

`teleport-other.yaml`:
```yaml
teleport:
  auth_servers: auth.example.com:3025

kubernetes_service:
  enabled: yes
  kube_cluster_name: other.example.com
  # Note: public_addr/listen_addr are needed for the proxy_service to connect
  # to this kubernetes_service
  public_addr: other.example.com:3026
  listen_addr: 0.0.0.0:3026
```

```sh
$ tsh kube ls
Cluster Name       Status
-------------      ------
example.com        online
other.example.com  online
```

Next, user connects a third k8s cluster over a reverse tunnel to the proxy:

`teleport-third.yaml`:
```yaml
teleport:
  auth_servers: proxy.example.com:3080

kubernetes_service:
  enabled: yes
  kube_cluster_name: third.example.com
  # Note: public_addr/listen_addr are NOT needed because this
  # kubernetes_service connects to proxy_service over a reverse tunnel (via
  # auth_servers address above).
```

```sh
$ tsh kube ls
Cluster Name       Status
-------------      ------
example.com        online
other.example.com  online
third.example.com  online
```

#### Scenario 2

A single central Teleport instance connected to multiple k8s clusters.

`teleport.yaml`:
```yaml
proxy_service:
  public_addr: proxy.example.com:3080
  kube_listen_addr: 0.0.0.0:3026

kubernetes_service:
  enabled: yes
  kubeconfig_file: kubeconfig
```

`kubeconfig`:
```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    server: https://a.example.com
  name: a-cluster
- cluster:
    server: https://b.example.com
  name: b-cluster
contexts:
- context:
    cluster: a-cluster
    user: shared-creds
  name: a
- context:
    cluster: b-cluster
    user: shared-creds
  name: b
current-context: a
users:
- name: shared-creds
  user:
    client-key-data: "..."
    client-certificate-data: "..."
```

```sh
$ tsh kube ls
Cluster Name       Status
-------------      ------
a                  online
b                  online
```

#### Scenario 3

A single central Teleport proxy acting as "gateway". Multiple k8s clusters
connect to it over reverse tunnels.

`teleport-proxy.yaml`:
```yaml
proxy_service:
  public_addr: proxy.example.com:3080
  kube_listen_addr: 0.0.0.0:3026
```

`teleport-a.yaml`:
```yaml
teleport:
  auth_servers: proxy.example.com:3080

kubernetes_service:
  enabled: yes
  kube_cluster_name: a.example.com
```

`teleport-b.yaml`:
```yaml
teleport:
  auth_servers: proxy.example.com:3080

kubernetes_service:
  enabled: yes
  kube_cluster_name: b.example.com
```

```sh
$ tsh kube ls
Cluster Name       Status
-------------      ------
a.example.com      online
b.example.com      online
```
