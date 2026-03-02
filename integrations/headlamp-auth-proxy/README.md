# Headlamp Auth Proxy for Teleport

A sidecar proxy that bridges Teleport app access identity to Kubernetes RBAC
via impersonation, enabling per-user permissions in [Headlamp](https://headlamp.dev/)
without token prompts or Kubernetes API server changes.

## How It Works

The auth-proxy runs as a sidecar alongside Headlamp with two proxy roles:

1. **Front proxy (:4466)** — Verifies the Teleport JWT signature against JWKS
   public keys, mints an internal HMAC token encoding the user identity, injects
   it as a Headlamp session cookie and `X-Auth-Token` header, and forwards to
   Headlamp (:4467).

2. **K8s API proxy (:6443)** — Receives requests from Headlamp bearing the HMAC
   token, validates it, and forwards to the real K8s API with
   `Impersonate-User`/`Impersonate-Group` headers.

## Installation

The integration uses the official Headlamp Helm chart with a values overlay —
no custom chart or modifications to the Teleport agent chart needed.

### Prerequisites

- A running Teleport cluster with app and discovery access
- A Kubernetes cluster with `kubectl` and `helm` access
- The `teleport-kube-agent` chart installed with `roles: app,kube,discovery`

### 1. Install Headlamp with the values overlay

```bash
helm repo add headlamp https://kubernetes-sigs.github.io/headlamp/
helm install headlamp headlamp/headlamp \
  --namespace headlamp \
  --create-namespace \
  --set teleportProxyAddr=https://example.cloud.gravitational.io \
  -f integrations/headlamp-auth-proxy/teleport-headlamp-values.yaml
```

The `teleportProxyAddr` value is required. It is the public URL of your
Teleport proxy (used to fetch JWKS public keys for JWT verification).

### 2. Access Headlamp

Teleport's discovery service auto-discovers the Headlamp Service (via the
`teleport.dev/name: headlamp` annotation) and registers it as an application.

Open: `https://headlamp.<your-teleport-proxy>/`

No token prompt — users are authenticated via their Teleport identity.
