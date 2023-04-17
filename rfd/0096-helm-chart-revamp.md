---
authors: Hugo Hervieux (hugo.hervieux@goteleport.com)
state: implemented (v12.0)
---

# RFD 0096 - Helm chart revamping

## Required approvers

* Engineering: @r0mant && @tigrato && ( @gus || @programmerq )
* Product: @klizhentas || @xinding33
* Security: @reedloden

## What

This proposal describes structural changes made to the Teleport Helm charts to
achieve the following goals:

- deploy auth and proxies separately
- reduce the time to deploy in most common setups (`aws` and `standalone`)
- always support the latest Teleport features by default (reduce time-to-market)
- reduce the cost of chart maintenance
- ensure seamless updates between Teleport versions
- ensure out of the box configuration supports large scale deployments

## Why

Most self-hosted Teleport setups rely either on Helm charts or Terraform to
deploy and operate Teleport. We want those two methods to become reference
ways of deploying Teleport, providing out of the box the most secure and
available setup.

Helm charts should allow users to easily benefit from the best Teleport
deployment they can have.  This includes but does not limit to:
- security
- maintainability
- availability
- scalability

In its current state, the Helm chart deploys a all-in-one set of pods assuming
proxy, auth, and kubernetes-access roles. Splitting responsibilities across
multiple sets of pods would increase availability, scalability, and reduce
attack surface.

Helm charts are also lagging behind upstream Teleport in terms of feature.  The
`teleport-cluster` chart configuration is exposing a subset of the supported
`teleport.yaml` values, but under different names. This causes unnecessary
friction for the user and increases the cost of maintaining the chart
configuration template.

## Details

This proposal starts by discussing the chart structure and deployed resources.
The second part is dedicated to the chart values, configuration format, and
backward compatibility. The third part addresses new update strategy
constraints between major Teleport versions.

### Chart structure and deployed resources

The resources in the chart would be split in two subdirectories,
`templates/auth/` and `templates/proxy/` to clearly identify which resource is
used by which teleport node. Common resources should be put in `templates/`.

The chart would deploy two Deployments: one for the proxies and one for the auths.

- the `teleport-proxy` Deployment: Those pods are stateless by default and can
  be upscaled even in standalone mode. Deploying those nodes using a Deployment
  means we cannot mount persistent storage on them. As Teleport does not support
  graceful shutdown with record shipping, users might lose active sessions
  recordings during a rollout if using the `proxy` mode. Teleport nodes
  are relying on `kube` ProvisionTokens to join the auth nodes on startup ([see
  RFD-0094](https://github.com/gravitational/teleport/blob/rfd/0096-helm-chart-revamp/rfd/0096-helm-chart-revamp.md)).
- the `teleport-auth` Deployment:  Those pods cannot be
  replicated without remote backend for state and audit logs. When persistence is
  enabled, a single volume will be mounted to those pods and the update strategy
  will be "re-create". For setups in which auth pods are stateless, the Deployment
  can be scaled up.

The main LB service should send traffic to the proxies, two additional services
for in-cluster communication should be created: one for the proxies and one for
the auth.

The trust between auth and proxy should be bootstrapped by
[creating a provisionToken on start](https://github.com/gravitational/teleport/pull/19009).

#### Labels and selectors

Deploying different pod sets requires a way to discriminate them. The only
label set currently is `app: {{.Release.Name }}`. We should follow [Helm
label recommendations](https://helm.sh/docs/chart_best_practices/labels/):

| Label                        | Value                                 | Purpose   |
|------------------------------|---------------------------------------|-----------|
| app.kubernetes.io/name       | `{{- default .Chart.Name .Values.nameOverride \| trunc 63 \| trimSuffix "-" }}` | Identify the application. |
| helm.sh/chart                | `{{ .Chart.Name }}-{{ .Chart.Version \| replace "+" "_" }}` | This should be the chart name and version. |
| app.kubernetes.io/managed-by | `{{ .Release.Service }}`              | It is for finding all things managed by Helm. |
| app.kubernetes.io/instance   | `{{ .Release.Name }}`                 | It aids in differentiating between different instances of the same application. |
| app.kubernetes.io/version    | `{{ .Chart.AppVersion }}`             | The version of the app. |
| app.kubernetes.io/component  | Name of the main Teleport service: `auth`, `proxy`, `kube` | This describes which Teleport component is deployed. |

Those labels should be applied to all deployed resources when applicable.
This includes but does not limit to Pods, Deployments, ConfigMaps,
Secrets and Services.

Note: if multiple components are deployed in the same pod (e.g. auth and kube),
only the main component should appear in the `app.kubernetes.io/component`.
This avoids the label selectors to change when services are added or removed.

The `app: {{.Release.Name}}` label should stay on the auth pods for
compatibility reasons.

#### Monitoring

A single optional
[`PodMonitor`](https://github.com/prometheus-operator/prometheus-operator/blob/main/Documentation/api.md#podmonitor)
should be deployed per Helm release, selecting all pods based
on `app.kubernetes.io/name`.

#### Custom Resources

It was initially planned to allow deployment of custom resources through the chart.
Unfortunately, Helm does not support deploying both a CRD and its CRs in the same
release (it checks if the API is supported before deploying). This section has been
removed from the RFD during implementation.

### Values and Teleport configuration

#### Generating `teleport.yaml`

The Helm chart would still expose modes (`aws`, `gcp`, `standalone`, `custom`),
but allow users to pass arbitrary additional configuration or perform specific
overrides.  This way, users would not have to leave the happy path if they need
to set one specific value.  Manually implementing all configuration knobs in
Helm adds no value and brings confusion as some values are not supported or not
named the same way than the `teleport.yaml` field they set.

By leveraging Helm's templating functions `toYaml`, `fromYaml`, and sprig's
`mustMergeOverwrite` the charts would merge their automatically-generated
`teleport.yaml` with user-provided `teleport.yaml`.

A user deploying the `auth_service` chart in `standalone` mode and wanting to
set key exchange algorithms, remove extra log fields, and override kube
cluster's name would use the values:

```yaml
clusterName: my-cluster
chartMode: standalone
auth:
  teleportConfig:
    teleport:
      kex_algos:
        - ecdh-sha2-nistp256
        - ecdh-sha2-nistp384
        - ecdh-sha2-nistp521
      log:
        format:
          extra_fields: ~
    kubernetes_service:
      kube_cluster_name: my-override
```

The generated chart configuration for the `standalone` mode is
```yaml
teleport:
  log:
    severity: INFO
    output: stderr
    format:
      output: text
      extra_fields: ["timestamp","level","component","caller"]
auth_service:
  enabled: true
  cluster_name: my-cluster
  authentication:
    type: "local"
    local_auth: true
    second_factor: "otp"
kubernetes_service:
  enabled: true
  listen_addr: 0.0.0.0:3027
  kube_cluster_name: my-cluster
proxy_service:
  enabled: false
ssh_service:
  enabled: false
```

Once merged with the custom user configuration, the resulting configuration is
```yaml
auth_service:
  authentication:
    local_auth: true
    second_factor: otp
    type: local
  cluster_name: my-cluster
  enabled: true
kubernetes_service:
  enabled: true
  kube_cluster_name: my-override
  listen_addr: 0.0.0.0:3027
proxy_service:
  enabled: false
ssh_service:
  enabled: false
teleport:
  kex_algos:
    - ecdh-sha2-nistp256
    - ecdh-sha2-nistp384
    - ecdh-sha2-nistp521
  log:
    format:
      extra_fields: null
      output: text
    output: stderr
    severity: INFO
```

The proof of concept code [can be found
here](https://github.com/hugoShaka/teleport-helm-config-poc).

The main drawback of this approach is that comments and value ordering are lost
during the round-trip.  This approach could be extended to support multiple
configuration syntax, following a breaking change for example.

`custom` should be removed in favor of a new `scratch` mode. Compared to
the previous Helm chart, users would not provide an external ConfigMap
but pass the custom configuration through the values. This is a breaking
change for them, but by the nature of the auth/proxy split it is not possible
to be backward compatible with `custom` mode.

In order to mitigate the risk of building an invalid configuration, the chart should
run pre-install and pre-upgrade hooks validating the configuration.

#### Backward compatibility

Splitting between auth and proxies will imply breaking some logic, we will try
to provide backward compatibility as much as possible. This includes being
compatible with the previous installation guides and seamlessly upgrading setups
created from those guides.

The revamp of the `teleport-cluster` change should ensure the IP of the service
stays the same, this requires the loadbalancing service to remain the same.

#### Teleport-specific configuration values

This proposal introduces two new values for users to edit the `teleport.yaml`
config: `auth.teleportConfig` and `proxy.teleportConfig`. The content of those
values should be merged with the generated configuration, as described [in the
previous section](#generating-teleportyaml).

For example:
```yaml
auth:
  teleportConfig:
    auth_service:
      authentication:
        connector_name: "my-connector"
proxy:
  teleportConfig:
    proxy_service:
      acme:
        enabled: true
        email: foo@example.com
```

The following values are core values: users must set them for the chart to work
properly. They support the happy path. Those values should not be changed by
the proposal as it would harm backward compatibility and user experience.

- `clusterName`
- `publicAddr`
- `chartMode`
- `aws`
- `gcp`
- `enterprise`
- `operator`

The following values are used to generate Teleport's configuration, we must
continue to support them for backward compatibility, but using
`*.teleportConfig` should be preferred.

- `kubeClusterName`
- `authentication`
- `authenticationType`
- `authenticationSecondFactor`
- `proxyListenerMode`
- `sessionRecording`
- `separatePostgresListener`
- `separateMongoListener`
- `kubePublicAddr`
- `mongoPublicAddr`
- `mysqlPublicAddr`
- `postgresPublicAddr`
- `sshPublicAddr`
- `tunnelPublicAddr`
- `acme`
- `acmeEmail`
- `acmeURI`
- `log`

#### Chart-specific configuration values

Some values are used to configure the Kubernetes resources deploying Teleport.
When specified they should apply to both auth and proxy deployments.  Those
values are:

- `podSecurityPolicy`
- `labels`
- `highAvailability`
- `tls`
- `image`
- `enterpriseImage`
- `affinity`
- `annotations`
- `extraArgs`
- `extraEnv`
- `extraVolumes`
- `extraVolumeMounts`
- `imagePullPolicy`
- `initContainers`
- `postStart`
- `securityContext`
- `priorityClassName`
- `tolerations`
- `probeTimeoutSeconds`
- `teleportVersionOverride`
- `resources`

A few values will have to be treated differently:

- `persistence` will only apply to the `auth` deployment
- `service` will only apply to the `proxy` service
- `serviceAccount.name` will apply to the `auth`, the proxy service account
  name should be the auth one suffixed with `-proxy`

Some users will need to set different values for auth and proxy pods, the
following values should be also available under `auth` and `proxy`. Those
specific values should take precedence over the ones at the root.

- `labels`
- `highAvailability` (except the certManager section)
- `affinity`
- `annotations`
- `extraArgs`
- `extraEnv`
- `extraVolumes`
- `extraVolumeMounts`
- `initContainers`
- `postStart`
- `tolerations`
- `teleportVersionOverride`
- `resources`

### Configuration examples

As this RFD brings numerous value changes and adds several ways of doing the same
thing, users should be provided full working examples covering various common setups.

Such examples would complete the documentation by demonstrating to users the best
practices and capabilities of the chart.

Those examples should also be used to lint the chart.

### Update strategy between major versions

Auth pods have to be updated before proxies.  Helm does not support applying
resources in a specific order.

Both auth and proxy rollouts will be triggered at the same time, but the proxy
one should be held until all auth pods are rolled out. Not waiting for the full
rollout will cause the load to spread unevenly across auth pods, which will be
harmful at scale.

Proxies will have an initContainer checking if all auth pods from the past
version were removed. Version check via the Teleport gRPC api (`PingResponse`)
requires valid credentials to connect to Teleport. To work around this issue we
can rely on Kubernetes's service discovery through DNS to discover how many
pods are running which version:

- the chart labels auth pods with their major teleport version
- the chart creates two headless services:
  - one selecting pods with the current major version `teleport-auth-v11`
  - one selecting pods with the previous major version `teleport-auth-v10`
- proxy pods have an initContainer
- the `v11` initContainer resolves `teleport-auth-v10` every 5 seconds until no
  IP is returned
- the initContainer exits, the proxy starts
- this unlocks the proxy deployment rollout

Headless services selecting auth pods with a specific version should contain
on-ready endpoints to ensure the rollout happens only when all pods are
completely terminated. This means setting `spec.publishNotReadyAddresses: true`.

This rollout approach might take some time on largest Teleport deployments.
This is not an issue per-se but has to be documented, as users running with
`--atomic` or `--wait` might have to increase their Helm timeouts.

Note: Teleport does not officially support multiple auth nodes running under
different major versions. The recommended update approach is to scale down to
a single node, update, and scale back up. In reality, most Teleport versions
are backward compatible with the previous major version, running multiple auth
is rarely an issue. This potential issue seems more related to Teleport than to
the deployment method, it will be considered out of scope of this RFD for the
sake of simplicity.
