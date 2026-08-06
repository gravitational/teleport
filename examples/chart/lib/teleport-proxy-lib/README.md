# teleport-proxy-lib

Library chart to deploy the Teleport proxy.

See `examples/chart/lib/README.md` for the repo-wide library-chart conventions.

## Public template

### `teleport-proxy-lib.all`

Renders the manifests to deploy a Teleport proxy as a multi-document YAML
stream; manifests for disabled features are omitted. Call it with the consumer's
root context:

```
{{- include "teleport-proxy-lib.all" . -}}
```

Inputs are read from top-level `.Values.*` (plus `.Release`, `.Chart`,
`.Capabilities`).

## Values

- `acme`, `acmeEmail`, `acmeURI` — obtain the proxy certificate via ACME/Let's
  Encrypt
  ([`proxy_service.acme`](https://goteleport.com/docs/reference/deployment/config/#proxy-service));
  mutually exclusive with `tls.existingSecretName`.
- `annotations` — per-manifest
  [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/),
  keyed by manifest (`config`, `deployment`, `pod`, `service`, `serviceAccount`,
  `ingress`, `certSecret`); applied only to the matching manifest.
- `chartMode` — config generation: `standalone` (full proxy config) or `scratch`
  (minimal — supply the rest via `teleportConfig`).
- `clusterName` — proxy hostname; default for
  [`proxy_service.public_addr`](https://goteleport.com/docs/reference/deployment/config/#proxy-service)
  (`clusterName:443`) and cert-manager SAN. Must not contain a port.
- `extraArgs` — extra `teleport` CLI args (container
  [`args`](https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/)).
- `extraContainers` —
  [sidecar containers](https://kubernetes.io/docs/concepts/workloads/pods/sidecar-containers/).
- `extraEnv` — extra container
  [environment variables](https://kubernetes.io/docs/tasks/inject-data-application/define-environment-variable-container/).
- `extraLabels` — per-manifest extra
  [labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/),
  keyed like `annotations` (plus `job`, `jobPod`, `podDisruptionBudget`).
- `extraVolumes`, `extraVolumeMounts` — extra pod
  [volumes](https://kubernetes.io/docs/concepts/storage/volumes/) / mounts.
- `goMemLimitRatio` — sets `GOMEMLIMIT` to memory-limit × ratio
  ([Go runtime](https://pkg.go.dev/runtime#hdr-Environment_Variables)).
- `highAvailability.certManager` — issue the proxy cert via
  [cert-manager](https://cert-manager.io/docs/)
  (`issuerName`/`issuerKind`/`issuerGroup`, `addCommonName`, `addPublicAddrs`).
- `highAvailability.minReadySeconds` — Deployment
  [`minReadySeconds`](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#min-ready-seconds).
- `highAvailability.podDisruptionBudget` — create a
  [PodDisruptionBudget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/).
- `highAvailability.replicaCount`, `forceHAReplicas` — replica count; replicable
  deployments (cert-manager / existing TLS secret / ingress) are bumped to ≥2
  unless `forceHAReplicas` honors the count as-is.
- `highAvailability.requireAntiAffinity` — require pods on separate nodes (hard
  [anti-affinity](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/#affinity-and-anti-affinity)).
- `image`, `enterpriseImage`, `enterprise` — container
  [image](https://kubernetes.io/docs/concepts/containers/images/);
  `enterprise: true` selects `enterpriseImage`. Tag comes from
  `teleportVersionOverride`/chart version.
- `imagePullPolicy`, `imagePullSecrets` —
  [image pull](https://kubernetes.io/docs/concepts/containers/images/) policy
  and secrets.
- `ingress` — optionally create an
  [Ingress](https://kubernetes.io/docs/concepts/services-networking/ingress/)
  routing to the Service (requires `proxyListenerMode: multiplex`); sub-keys
  `useExisting`, `suppressAutomaticWildcards`, `spec`.
- `initContainers` — extra
  [init containers](https://kubernetes.io/docs/concepts/workloads/pods/init-containers/)
  (Default teleport volume mounts are added unless `skipDefaultVolumeMounts` key
  is set. This non-k8s key is revmoed. All other keys are standard k8s
  init-container YAML.)
- `insecureSkipProxyTLSVerify` — adds `--insecure` to the `teleport` args; skips
  proxy TLS certificate verification (dev only).
- `jobResources` —
  [resources](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)
  for the `validateConfigOnDeploy` Job.
- `kubePublicAddr` —
  [`proxy_service.kube_public_addr`](https://goteleport.com/docs/reference/deployment/config/#proxy-service).
- `log`, `logLevel` —
  [`teleport.log`](https://goteleport.com/docs/reference/deployment/config/#instance-wide-settings)
  severity/output/format; `logLevel` is a legacy alias for `log.level`.
- `mongoPublicAddr` —
  [`proxy_service.mongo_public_addr`](https://goteleport.com/docs/reference/deployment/config/#proxy-service).
- `mysqlPublicAddr` —
  [`proxy_service.mysql_public_addr`](https://goteleport.com/docs/reference/deployment/config/#proxy-service).
- `nameOverride` — overrides the `app.kubernetes.io/name` label.
- `nodeSelector`, `affinity`, `tolerations` — pod
  [scheduling](https://kubernetes.io/docs/concepts/scheduling-eviction/assign-pod-node/);
  setting `affinity` replaces the chart's default anti-affinity.
- `podSecurityContext`, `securityContext` — pod / container
  [security context](https://kubernetes.io/docs/tasks/configure-pod-container/security-context/).
- `postgresPublicAddr` —
  [`proxy_service.postgres_public_addr`](https://goteleport.com/docs/reference/deployment/config/#proxy-service).
- `postStart` — container `postStart`
  [lifecycle hook](https://kubernetes.io/docs/tasks/configure-pod-container/attach-handler-lifecycle-event/)
  command.
- `priorityClassName` — pod
  [priority class](https://kubernetes.io/docs/concepts/scheduling-eviction/pod-priority-preemption/).
- `probeTimeoutSeconds`, `readinessProbe` — liveness/readiness
  [probe](https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/)
  timeout and readiness thresholds.
- `proxyListenerMode` — `multiplex` (single TLS port) vs `separate` per-protocol
  listeners
  ([TLS routing](https://goteleport.com/docs/reference/architecture/tls-routing/));
  drives the listeners/ports in the config, Deployment, and Service.
- `proxyProtocol` — sets
  [`proxy_service.proxy_protocol`](https://goteleport.com/docs/reference/deployment/config/#proxy-service).
- `publicAddr` —
  [`proxy_service.public_addr`](https://goteleport.com/docs/reference/deployment/config/#proxy-service)
  (defaults to `clusterName:443`).
- `resources` — main container
  [resources](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/)
  (also the default for `initContainers`).
- `separateMongoListener` — expose a dedicated Mongo listener
  ([`proxy_service.mongo_listen_addr`](https://goteleport.com/docs/reference/deployment/config/#proxy-service)
  - port 27017) instead of multiplexing.
- `separatePostgresListener` — expose a dedicated Postgres listener
  ([`proxy_service.postgres_listen_addr`](https://goteleport.com/docs/reference/deployment/config/#proxy-service)
  - Service/Deployment port 5432) instead of multiplexing.
- `service`, `service.type` — the
  [Service](https://kubernetes.io/docs/concepts/services-networking/service/)
  that exposes the proxy (default `LoadBalancer`); `service.spec` is merged in.
  Always created — the primary way to reach the proxy.
- `serviceAccount` — `create` the
  [ServiceAccount](https://kubernetes.io/docs/concepts/security/service-accounts/)
  and its `name` (used verbatim — no suffix appended).
- `sshPublicAddr` —
  [`proxy_service.ssh_public_addr`](https://goteleport.com/docs/reference/deployment/config/#proxy-service).
- `teleportAuthService` — sets
  [`teleport.auth_server`](https://goteleport.com/docs/reference/deployment/config/#instance-wide-settings).
- `teleportConfig` — raw
  [`teleport.yaml`](https://goteleport.com/docs/reference/deployment/config/)
  overrides, deep-merged over the generated config.
- `teleportVersionOverride` — image tag / Teleport version (defaults to the
  chart version).
- `terminationGracePeriodSeconds` — pod
  [termination grace period](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination).
- `tls.existingCASecretName`, `tls.existingCASecretKeyName` — mount an extra CA
  Secret and point `SSL_CERT_FILE` at the named key (trust a private CA).
- `tls.existingSecretName` — mount an existing TLS Secret as the proxy keypair
  ([`proxy_service.https_keypairs`](https://goteleport.com/docs/reference/deployment/config/#proxy-service))
  instead of ACME/cert-manager.
- `tunnelPublicAddr` —
  [`proxy_service.tunnel_public_addr`](https://goteleport.com/docs/reference/deployment/config/#proxy-service).
- `validateConfigOnDeploy` — run a pre-install/pre-upgrade
  [Helm hook](https://helm.sh/docs/topics/charts_hooks/) Job that executes
  `teleport configure --test` on the rendered config before rollout.
