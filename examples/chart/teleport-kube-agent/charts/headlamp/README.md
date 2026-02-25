# Headlamp Helm Chart

Headlamp is an easy-to-use and extensible Kubernetes web UI that provides:
- 🚀 Modern, fast, and responsive interface
- 🔒 OIDC authentication support
- 🔌 Plugin system for extensibility
- 🎯 Real-time cluster state updates

## Prerequisites

- Kubernetes 1.21+
- Helm 3.x
- Cluster admin access for initial setup

## Quick Start

Add the Headlamp repository and install the chart:

```console
$ helm repo add headlamp https://kubernetes-sigs.github.io/headlamp/
$ helm repo update
$ helm install my-headlamp headlamp/headlamp --namespace kube-system
```

Access Headlamp:
```console
$ kubectl port-forward -n kube-system svc/my-headlamp 8080:80
```
Then open http://localhost:8080 in your browser.

## Installation

### Basic Installation
```console
$ helm install my-headlamp headlamp/headlamp --namespace kube-system
```

### Installation with OIDC
```console
$ helm install my-headlamp headlamp/headlamp \
  --namespace kube-system \
  --set config.oidc.clientID=your-client-id \
  --set config.oidc.clientSecret=your-client-secret \
  --set config.oidc.issuerURL=https://your-issuer-url
```

### Installation with Ingress
```console
$ helm install my-headlamp headlamp/headlamp \
  --namespace kube-system \
  --set ingress.enabled=true \
  --set ingress.hosts[0].host=headlamp.example.com \
  --set ingress.hosts[0].paths[0].path=/
```

## Configuration

### Core Parameters

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| replicaCount | int | `1` | Number of desired pods |
| image.registry | string | `"ghcr.io"` | Container image registry |
| image.repository | string | `"headlamp-k8s/headlamp"` | Container image name |
| image.tag | string | `""` | Container image tag (defaults to Chart appVersion) |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |

### Application Configuration

| Key                | Type   | Default               | Description                                                               |
|--------------------|--------|-----------------------|---------------------------------------------------------------------------|
| config.inCluster   | bool   | `true`                | Run Headlamp in-cluster                                                   |
| config.baseURL     | string | `""`                  | Base URL path for Headlamp UI                                             |
| config.sessionTTL  | int    | `86400`               | The time in seconds for the internal session to remain valid (Default: 86400/24h, Min: 1 , Max: 31536000/1yr) |
| config.pluginsDir  | string | `"/headlamp/plugins"` | Directory to load Headlamp plugins from                                   |
| config.enableHelm  | bool   | `false`               | Enable Helm operations like install, upgrade and uninstall of Helm charts |
| config.extraArgs   | array  | `[]`                  | Additional arguments for Headlamp server                                  |
| config.tlsCertPath | string | `""`                  | Certificate for serving TLS                                               |
| config.tlsKeyPath  | string | `""`                  | Key for serving TLS                                                       |

### OIDC Configuration

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| config.oidc.clientID | string | `""` | OIDC client ID |
| config.oidc.clientSecret | string | `""` | OIDC client secret |
| config.oidc.issuerURL | string | `""` | OIDC issuer URL |
| config.oidc.scopes | string | `""` | OIDC scopes to be used |
| config.oidc.usePKCE | bool | `false` | Use PKCE (Proof Key for Code Exchange) for enhanced security in OIDC flow |
| config.oidc.secret.create | bool | `true` | Create OIDC secret using provided values |
| config.oidc.secret.name | string | `"oidc"` | Name of the OIDC secret |
| config.oidc.externalSecret.enabled | bool | `false` | Enable using external secret for OIDC |
| config.oidc.externalSecret.name | string | `""` | Name of external OIDC secret |
| config.oidc.meUserInfoURL | string | `""` | URL to fetch additional user info for the /me endpoint. Useful for providers like oauth2-proxy. |

There are three ways to configure OIDC:

1. Using direct configuration:
```yaml
config:
  oidc:
    clientID: "your-client-id"
    clientSecret: "your-client-secret"
    issuerURL: "https://your-issuer"
    scopes: "openid profile email"
    meUserInfoURL: "https://headlamp.example.com/oauth2/userinfo"
```

2. Using automatic secret creation:
```yaml
config:
  oidc:
    secret:
      create: true
      name: oidc
```

3. Using external secret:
```yaml
config:
  oidc:
    secret:
      create: false
    externalSecret:
      enabled: true
      name: your-oidc-secret
```

### Deployment Configuration

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| replicaCount | int | `1` | Number of desired pods |
| image.registry | string | `"ghcr.io"` | Container image registry |
| image.repository | string | `"headlamp-k8s/headlamp"` | Container image name |
| image.tag | string | `""` | Container image tag (defaults to Chart appVersion) |
| image.pullPolicy | string | `"IfNotPresent"` | Image pull policy |
| imagePullSecrets | list | `[]` | Image pull secrets references |
| nameOverride | string | `""` | Override the name of the chart |
| fullnameOverride | string | `""` | Override the full name of the chart |
| namespaceOverride | string | `""` | Override the deployment namespace; defaults to .Release.Namespace |
| initContainers | list | `[]` | Init containers to run before main container |

### Security Configuration

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| automountServiceAccountToken | bool | `true` | Mount Service Account token in pod |
| serviceAccount.create | bool | `true` | Create service account |
| serviceAccount.name | string | `""` | Service account name |
| serviceAccount.annotations | object | `{}` | Service account annotations |
| clusterRoleBinding.create | bool | `true` | Create cluster role binding |
| clusterRoleBinding.clusterRoleName | string | `"cluster-admin"` | Kubernetes ClusterRole name |
| clusterRoleBinding.annotations | object | `{}` | Cluster role binding annotations |
| hostUsers | bool | `true` | Run in host uid namespace |
| podSecurityContext | object | `{}` | Pod security context (e.g., fsGroup: 2000) |
| securityContext.runAsNonRoot | bool | `true` | Run container as non-root |
| securityContext.privileged | bool | `false` | Run container in privileged mode |
| securityContext.runAsUser | int | `100` | User ID to run container |
| securityContext.runAsGroup | int | `101` | Group ID to run container |
| securityContext.capabilities | object | `{}` | Container capabilities (e.g., drop: [ALL]) |
| securityContext.readOnlyRootFilesystem | bool | `false` | Mount root filesystem as read-only |

NOTE: for `hostUsers=false` user namespaces must be supported. See: https://kubernetes.io/docs/concepts/workloads/pods/user-namespaces/

### Storage Configuration

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| persistentVolumeClaim.enabled | bool | `false` | Enable PVC |
| persistentVolumeClaim.annotations | object | `{}` | PVC annotations |
| persistentVolumeClaim.size | string | `""` | PVC size (required if enabled) |
| persistentVolumeClaim.storageClassName | string | `""` | Storage class name |
| persistentVolumeClaim.accessModes | list | `[]` | PVC access modes |
| persistentVolumeClaim.selector | object | `{}` | PVC selector |
| persistentVolumeClaim.volumeMode | string | `""` | PVC volume mode |
| volumeMounts | list | `[]` | Container volume mounts |
| volumes | list | `[]` | Pod volumes |

### Network Configuration

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| service.type | string | `"ClusterIP"` | Kubernetes service type |
| service.port | int | `80` | Kubernetes service port |
| ingress.enabled | bool | `false` | Enable ingress |
| ingress.ingressClassName | string | `""` | Ingress class name |
| ingress.annotations | object | `{}` | Ingress annotations (e.g., kubernetes.io/tls-acme: "true") |
| ingress.labels | object | `{}` | Additional labels for the Ingress resource |
| ingress.hosts | list | `[]` | Ingress hosts configuration |
| ingress.tls | list | `[]` | Ingress TLS configuration |

Example ingress configuration:
```yaml
ingress:
  enabled: true
  annotations:
    kubernetes.io/tls-acme: "true"
  labels:
    app.kubernetes.io/part-of: traefik
    environment: prod
  hosts:
    - host: headlamp.example.com
      paths:
        - path: /
          type: ImplementationSpecific
  tls:
    - secretName: headlamp-tls
      hosts:
        - headlamp.example.com
```

### HTTPRoute Configuration (Gateway API)

For users who prefer Gateway API over classic Ingress resources, Headlamp supports HTTPRoute configuration.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| httpRoute.enabled | bool | `false` | Enable HTTPRoute resource for Gateway API |
| httpRoute.annotations | object | `{}` | Annotations for HTTPRoute resource |
| httpRoute.labels | object | `{}` | Additional labels for HTTPRoute resource |
| httpRoute.parentRefs | list | `[]` | Parent gateway references (REQUIRED when enabled) |
| httpRoute.hostnames | list | `[]` | Hostnames for the HTTPRoute |
| httpRoute.rules | list | `[]` | Custom routing rules (optional, defaults to path prefix /) |

Example HTTPRoute configuration:
```yaml
httpRoute:
  enabled: true
  annotations:
    gateway.example.com/custom-annotation: "value"
  labels:
    app.kubernetes.io/component: ingress
  parentRefs:
    - name: my-gateway
      namespace: gateway-namespace
  hostnames:
    - headlamp.example.com
  # Optional custom rules (defaults to path prefix / if not specified)
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /headlamp
      backendRefs:
        - name: my-headlamp
          port: 80
```

### Resource Management

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| resources | object | `{}` | Container resource requests/limits |
| nodeSelector | object | `{}` | Node labels for pod assignment |
| tolerations | list | `[]` | Pod tolerations |
| affinity | object | `{}` | Pod affinity settings |
| topologySpreadConstraints | list | `[]` | Topology spread constraints for pod assignment |
| podAnnotations | object | `{}` | Pod annotations |
| podLabels | object | `{}` | Pod labels |
| env | list | `[]` | Additional environment variables |

Example resource configuration:
```yaml
resources:
  limits:
    cpu: 100m
    memory: 128Mi
  requests:
    cpu: 100m
    memory: 128Mi
```

Example environment variables:
```yaml
env:
  - name: KUBERNETES_SERVICE_HOST
    value: "localhost"
  - name: KUBERNETES_SERVICE_PORT
    value: "6443"
```

Example topology spread constraints:
```yaml
# Spread pods across availability zones with best-effort scheduling
topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: topology.kubernetes.io/zone
    whenUnsatisfiable: ScheduleAnyway  # Prefer spreading but allow scheduling even if it violates the constraint
    matchLabelKeys:
      - pod-template-hash
  - maxSkew: 1
    topologyKey: kubernetes.io/hostname
    whenUnsatisfiable: DoNotSchedule  # Hard requirement - don't schedule if it violates the constraint
    matchLabelKeys:
      - pod-template-hash
```

The `labelSelector` is automatically populated with the pod's selector labels if not specified. You can also provide a custom `labelSelector`:
```yaml
topologySpreadConstraints:
  - maxSkew: 1
    topologyKey: topology.kubernetes.io/zone
    whenUnsatisfiable: ScheduleAnyway
    labelSelector:
      matchLabels:
        app.kubernetes.io/name: headlamp
        custom-label: value
```

### Pod Disruption Budget (PDB)

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| podDisruptionBudget.enabled | bool | `false` | Create a PodDisruptionBudget resource |
| podDisruptionBudget.minAvailable | integer \| string \| null | `0` | Minimum pods that must be available. Rendered only when set to a positive integer or a percentage string (e.g. `"1"` or `"50%"`). Schema default is 0, but the chart skips rendering `0`. |
| podDisruptionBudget.maxUnavailable | integer \| string \| null | `null` | Maximum pods allowed to be unavailable. Accepts integer >= 0 or percentage string. Mutually exclusive with `minAvailable`; the template renders this field when set. |
| podDisruptionBudget.unhealthyPodEvictionPolicy | string \| null | `null` | Eviction policy: `"IfHealthyBudget"` or `"AlwaysAllow"`. Emitted only on clusters running Kubernetes >= 1.27 and when explicitly set in values. |

Note: Ensure `minAvailable` and `maxUnavailable` are not both set (use `null` to disable one). To include `minAvailable` in the rendered PDB, set a positive integer or percentage; the template omits a `0` value.

Example, Require at least 1 pod available (ensure maxUnavailable is disabled):
```yaml
podDisruptionBudget:
  enabled: true
  minAvailable: 1
  maxUnavailable: null
```

Example, Allow up to 50% of pods to be unavailable:
```yaml
podDisruptionBudget:
  enabled: true
  maxUnavailable: "50%"
  minAvailable: null
```

Example, Set unhealthyPodEvictionPolicy (requires Kubernetes >= 1.27):
```yaml
podDisruptionBudget:
  enabled: true
  maxUnavailable: 1
  minAvailable: null
  unhealthyPodEvictionPolicy: "IfHealthyBudget"
```

Ensure your replicaCount and maintenance procedures respect the configured PDB to avoid blocking intended operations.

### pluginsManager Configuration

| Key           | Type    | Default           | Description                                                                               |
| ------------- | ------- | ----------------- | ----------------------------------------------------------------------------------------- |
| enabled       | boolean | `false`           | Enable plugin manager                                                                     |
| configFile    | string  | `plugin.yml`      | Plugin configuration file name                                                            |
| configContent | string  | `""`              | Plugin configuration content in YAML format. This is required if plugins.enabled is true. |
| baseImage     | string  | `node:lts-alpine` | Base node image to use                                                                    |
| version       | string  | `latest`          | Headlamp plugin package version to install                                                |
| env           | list    | `[]`              | Plugin manager env variable configuration                                                 |
| resources     | object  | `{}`              | Plugin manager resource requests/limits                                                   |
| volumeMounts  | list    | `[]`              | Plugin manager volume mounts                                                              |

Example resource configuration:

```yaml
pluginsManager:
  enabled: true
  baseImage: node:lts-alpine
  version: latest
  env:
    - name: HTTPS_PROXY
      value: "proxy.example.com:8080"
  resources:
    requests:
      cpu: "500m"
      memory: "2048Mi"
    limits:
      cpu: "1"
      memory: "4Gi"
```
## Contributing

We welcome contributions to the Headlamp Helm chart! To contribute:

1. Fork the repository and create your branch from `main`.
2. Make your changes and test them thoroughly.
3. Run Helm chart template tests to ensure your changes don't break existing functionality:

   ```console
   $ make helm-template-test
   ```
   This command executes the script at `charts/headlamp/tests/test.sh` to validate Helm chart templates against expected templates.

4. If you've made changes that intentionally affect the rendered templates (like version updates or new features):

   ```console
   $ make helm-update-template-version
   ```
   This updates the expected templates with the current versions from Chart.yaml and only shows files where versions changed.

5. Review the updated templates carefully to ensure they contain only your intended changes.

6. Submit a pull request with a clear description of your changes.


For more details, refer to our [contributing guidelines](https://github.com/kubernetes-sigs/headlamp/blob/main/CONTRIBUTING.md).

## Links

- [GitHub Repository](https://github.com/kubernetes-sigs/headlamp)
- [Documentation](https://headlamp.dev/)
- [Maintainers](https://github.com/kubernetes-sigs/headlamp/blob/main/OWNERS_ALIASES)
