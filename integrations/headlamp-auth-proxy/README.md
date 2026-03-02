# Headlamp Auth Proxy for Teleport

A sidecar proxy that bridges Teleport app access identity to Kubernetes RBAC
via impersonation, enabling per-user permissions in [Headlamp](https://headlamp.dev/)
without token prompts or Kubernetes API server changes.

## How It Works

The auth-proxy runs as a sidecar alongside Headlamp with two proxy roles:

1. **Front proxy (:4466)** — Decodes the Teleport JWT, mints an internal HMAC
   token encoding the user identity, injects it as a Headlamp session cookie and
   `X-Auth-Token` header, and forwards to Headlamp (:4467).

2. **K8s API proxy (:6443)** — Receives requests from Headlamp bearing the HMAC
   token, validates it, and forwards to the real K8s API with
   `Impersonate-User`/`Impersonate-Group` headers.

```
Service:80 -> pod:4466 (auth-proxy front) -> 127.0.0.1:4467 (Headlamp)
                                               | K8s API calls
                                            127.0.0.1:6443 (auth-proxy k8s) -> real K8s API
```

Requests without a Teleport JWT (e.g. Kubernetes health probes) are passed
through to Headlamp directly.

## Installation

The integration uses the official Headlamp Helm chart with a values overlay —
no custom chart or modifications to the Teleport agent chart needed.

### Prerequisites

- A running Teleport cluster with app and discovery access
- A Kubernetes cluster with `kubectl` and `helm` access
- The `teleport-kube-agent` chart installed with `roles: app,kube,discovery`

### 1. Build and push the auth-proxy image

```bash
cd integrations/headlamp-auth-proxy
docker build --platform linux/amd64 -t ghcr.io/jakealti/headlamp-auth-proxy:latest .
docker push ghcr.io/jakealti/headlamp-auth-proxy:latest
```

### 2. Install Headlamp with the values overlay

```bash
helm repo add headlamp https://headlamp-k8s.github.io/headlamp/
helm install headlamp headlamp/headlamp \
  --namespace teleport-agent \
  -f integrations/headlamp-auth-proxy/teleport-headlamp-values.yaml
```

Install into the same namespace as the Teleport agent so that Teleport's
discovery service can find the Headlamp Service.

### 3. Create RBAC for impersonated users

The auth-proxy impersonates Teleport users when calling the Kubernetes API.
Create RBAC bindings for the impersonated identities:

```bash
# Full access for a specific user
kubectl create clusterrolebinding headlamp-admin \
  --clusterrole=cluster-admin --user=<your-teleport-username>

# Read-only access for everyone with the "access" Teleport role
kubectl create clusterrolebinding headlamp-viewers \
  --clusterrole=view --group=access
```

### 4. Access Headlamp

Teleport's discovery service auto-discovers the Headlamp Service (via the
`teleport.dev/name: headlamp` annotation) and registers it as an application.

Open: `https://headlamp.<your-teleport-proxy>/`

No token prompt — users are authenticated via their Teleport identity.

## Configuration

Edit `teleport-headlamp-values.yaml` to customize:

| Setting | Location | Default | Description |
|---|---|---|---|
| Auth-proxy image | `extraContainers[0].image` | `ghcr.io/jakealti/headlamp-auth-proxy:latest` | Auth-proxy container image |
| Groups claim | `extraContainers[0].args` `--groups-claim` | `roles` | JWT claim for K8s groups (`roles` or `traits.<key>`) |
| Service annotation | `service.annotations` | `teleport.dev/name: headlamp` | Controls the Teleport app name |

## Files

| File | Purpose |
|---|---|
| `main.go` | Entry point, flags, signal handling, starts both proxy servers |
| `frontproxy.go` | Front proxy — decodes JWT, injects cookie + X-Auth-Token, forwards to Headlamp |
| `k8sproxy.go` | K8s proxy — validates internal token, adds impersonation headers |
| `jwks.go` | Decodes Teleport JWT payload (base64, no signature verification) |
| `token.go` | Mints/validates HMAC-signed internal JWTs |
| `Dockerfile` | Multi-stage container build |
| `teleport-headlamp-values.yaml` | Values overlay for the official Headlamp Helm chart |
