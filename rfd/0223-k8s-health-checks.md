---
authors: Rana Ian (rana.ian@goteleport.com)
state: draft
---

# RFD 0223 - Kubernetes Health Checks

## Required Approvals

- Engineering: @rosstimothy && @creack && @tigrato


## What

Enable automated Kubernetes cluster health checks that are viewed by the Web UI, Teleport Connect, `tctl` command, and Prometheus metrics.

## Why

Proxied Kubernetes cluster health may be manually exercised with Kubernetes operations in the Web UI, or `kubectl` command, then observing the result. While effective, manual checking is also slow and unscalable. 

Automated Kubernetes health checks with new methods of viewing improve maintainability of Teleport Kubernetes clusters, while enabling new scenarios.

Automated health checks:
- Improve time to observe and resolve Kubernetes cluster errors
- Improve manageability of Kubernetes clusters, especially at scale
- Improve the enrollment experience for Kubernetes clusters
- Enables alerting on unhealthy Kubernetes clusters
- Provides feature parity with databases and machine workload identities

## Details

### UX

#### User Story: Web UI - Enrolling Kubernetes Clusters

Alice enrolls three Amazon EKS clusters into Teleport through the Web UI.

The next day she returns to the Web UI _Resources_ tab to find the Amazon EKS clusters are highlighted with warnings. She clicks on an Amazon EKS tile and a health message is displayed in a side panel.

```
Kubernetes Cluster Issues

3 Teleport Kubernetes clusters report issues.

Affected Teleport Kubernetes cluster:
- Hostname: sol
  UUID: 52dedbd0-b165-4bf6-9bc3-961f95bf481d
  Error: 503 Service Unavailable
    [+]ping ok
    [+]log ok
    [-]etcd not ok: client: etcd cluster is unavailable or misconfigured: context deadline exceeded

Affected Teleport Kubernetes cluster:
- Hostname: jupiter
  UUID: bb4dc171-ffa7-4a31-ba8c-7bf91c59e250
  Error: 503 Service Unavailable
    [+]ping ok
    [+]log ok
    [-]etcd not ok: client: etcd cluster is unavailable or misconfigured: context deadline exceeded

Affected Teleport Kubernetes cluster:
- Hostname: saturn
  UUID: 2be08cb1-56a4-401f-a3f3-c755a73f3ff6
  Error: 503 Service Unavailable
    [+]ping ok
    [+]log ok
    [-]etcd not ok: client: etcd cluster is unavailable or misconfigured: context deadline exceeded
```

Alice notices that each cluster has a similar 503 message saying etcd is the source of the error.

Alice resolves the `etcd` error, and each Amazon EKS cluster returns to a healthy state.

As she monitors the Teleport Web UI, she sees each Amazon EKS tile switch from a warning state to a normal state.


#### User Story: `tctl` - Configuring a New Health Check

Bob reads about Kubernetes health checks in a Teleport changelog, and updates a Teleport cluster to the new major version.

Bob runs `tctl get health_check_config/default` from a terminal to view the default health settings.

```yaml
version: v1
metadata:
  name: "default"
  labels:
    teleport.internal/resource-type: preset
spec:
  match:
    db_labels:
      - name: "*"
        values:
          - "*"
    kubernetes_labels:
      - name: "*"
        values:
          - "*"
```

He notices a new `kubernetes_labels` matcher.

He vets the Kubernetes health checks in non-production environments.

Bob runs `tctl edit health_check_config/default` from a terminal, updating the default settings to exclude Kubernetes health checks from a production environment.

```yaml
version: v1
metadata:
  name: "default"
  labels:
    teleport.internal/resource-type: preset
spec:
  match:
    db_labels:
      - name: "*"
        values:
          - "*"
    kubernetes_labels:
      - name: "*"
        values:
          - "*"
    kubernetes_labels_expression: "labels.env != `prod`"
```

Bob runs `tctl get kube_server/luna` from a terminal, validating that the expected Kubernetes cluster is monitoring health. 
```yaml
kind: kube_server
metadata:
  expires: "2025-10-26T00:00:00.000000Z"
  name: luna
  revision: 43e96231-faaf-43c3-b9b8-15cf91813389
spec:
  host_id: 278be63c-c87e-4d7e-a286-86002c7c45c3
  hostname: luna
status:
  target_health:
    addr: luna:3027
    protocol: TLS
    transition_timestamp: "2025-10-25T00:00:00.000000Z"
    transition_reason: "healthy threshold reached"
    status: healthy
  version: 19.0.0
version: v3
```


#### User Story: Prometheus - Alerting on an Unhealthy Kubernetes Cluster

Charlie relies on Prometheus to notify him of outages and calls to action.

He reads about Kubernetes cluster health being available with Prometheus metrics.

Charlie tests the feature.

He enrolls three GKE instances into Teleport.

He queries the new Teleport Prometheus health metrics.
```promql
teleport_health_resources{type="kubernetes"}
# Returns 3, the expected number of healthy Kubernetes clusters

teleport_health_resources_available{type="kubernetes"}
# Returns 3, the actual number of healthy Kubernetes clusters
```

Charlie sets one GKE instance into an unhealthy state and requeries.

```promql
teleport_health_resources{type="kubernetes"}
# Returns 3, the expected number of healthy Kubernetes clusters

teleport_health_resources_available{type="kubernetes"}
# Returns 2, the actual number of healthy Kubernetes clusters
```

Seeing the metric values returning, he sets up a Prometheus alerting rule.

```yaml
groups:
  - name: teleport_kubernetes
    rules:
      - alert: KubernetesClusterUnhealthy
        expr: |
          (teleport_health_resources{type="kubernetes"} - 
           teleport_health_resources_available{type="kubernetes"}) > 0
        for: 5m
        labels:
          severity: warning
          team: platform
          component: teleport
          service: kubernetes
        annotations:
          summary: "{{ $value }} Kubernetes cluster(s) unhealthy in Teleport"
          description: "Teleport reports {{ $value }} unhealthy Kubernetes cluster(s). This indicates that one or more Kubernetes clusters registered with Teleport are not responding or failing health checks. Check Teleport Web UI or use tctl for details."
          runbook_url: "https://wiki.goteleport.com/runbooks/teleport-k8s-unhealthy"
          dashboard_url: "https://grafana.luna.com/d/teleport-k8s/teleport-kubernetes-health"
          query: 'teleport_health_resources{type="kubernetes"} - teleport_health_resources_available{type="kubernetes"}'
```

Prometheus alerts him about the unhealthy Kubernetes cluster.

Charlie sets the GKE instance into a healthy state and moves on with his day.


### Implementation Details

Kubernetes health checks are discussed by functional areas of core logic, `tctl` command, Web UI, and Prometheus metrics.

#### Core Implementation

Teleport Kubernetes health checks use the Teleport `healthcheck` package, and is based on existing [database health check](https://github.com/gravitational/teleport/blob/master/rfd/0203-database-healthchecks.md#user-content-fn-1-b6df2ad8fd7a63ee3ca0af227e74ab87) design patterns. The `healthcheck` components are written, tested, and in production. The focus and effort for Kubernetes health checks is integrating health checks into the Kubernetes agent, extending existing `healthcheck` mechanisms, and updating the UI.

##### Core Configuration

A first step to enabling Kubernetes health checks is adding new matchers to the `HealthCheckConfig` service. `HealthCheckConfig` identifies servers which choose to participate in health checking. `HealthCheckConfig` supports databases. Kubernetes additions mirror the database features.

The configuration adds matchers `kubernetes_labels` and `kubernetes_labels_expression` which specify labeled Kubernetes clusters. By default, all Kubernetes clusters participate in health checks. Matchers may filter Kubernetes clusters. Deleting the matchers excludes all Kubernetes clusters.

An example yaml `health_check_config`:
```yaml
version: v1
metadata:
  name: "default"
  labels:
    teleport.internal/resource-type: preset
spec:
  match:
    kubernetes_labels:
      - name: "*"
        values:
          - "*"
    kubernetes_labels_expression: "labels.env != `prod`"
```

Change points for health check configuration:
- [api/proto/teleport/healthcheckconfig/v1/health_check_config.proto](https://github.com/gravitational/teleport/blob/590c85a765a6d8d16d5f34503179f27a97a4625c/api/proto/teleport/healthcheckconfig/v1/health_check_config.proto#L59)
  - Adds `kubernetes_labels` and `kubernetes_labels_expression` to proto message `Matcher`
- [lib/services/health_check_config.go](https://github.com/gravitational/teleport/blob/590c85a765a6d8d16d5f34503179f27a97a4625c/lib/services/health_check_config.go#L58)
  - Adds Kubernetes label matcher validation to function `ValidateHealthCheckConfig()`
- [lib/healthcheck/config.go](https://github.com/gravitational/teleport/blob/590c85a765a6d8d16d5f34503179f27a97a4625c/lib/healthcheck/config.go#L35)
  - Adds Kubernetes label matcher field to type `healthCheckConfig`, and update functions `newHealthCheckConfig()` and `getLabelMatchers()`
- [lib/services/presets.go](https://github.com/gravitational/teleport/blob/590c85a765a6d8d16d5f34503179f27a97a4625c/lib/services/presets.go#L830)
  - Adds wildcard Kubernetes label matchers to function `NewPresetHealthCheckConfig()`

`HealthCheckConfig` is communicated via proxy and cached on a Kubernetes agent. Kubernetes interfaces are updated to support the communication and caching.

Change points for communication and caching:
- [lib/auth/authclient/api.go](https://github.com/gravitational/teleport/blob/590c85a765a6d8d16d5f34503179f27a97a4625c/lib/auth/authclient/api.go#L472)
  - Adds `services.HealthCheckConfigReader` to interfaces `ReadKubernetesAccessPoint` and `ProxyAccessPoint` 
- api/types/kubernetes_server.go
  - Adds functions `GetTargetHealth()`, `SetTargetHealth()`, `GetTargetHealthStatus()`, and `SetTargetHealthStatus()` to the `KubeServer` interface for implementing interface `services.HealthCheckConfigReader`
- [lib/cache/cache.go](https://github.com/gravitational/teleport/blob/590c85a765a6d8d16d5f34503179f27a97a4625c/lib/cache/cache.go#L356)
  - Adds watches for `types.KindHealthCheckConfig` in `ForKubernetes()` and `ForProxy()`
- [lib/authz/permissions.go](https://github.com/gravitational/teleport/blob/590c85a765a6d8d16d5f34503179f27a97a4625c/lib/authz/permissions.go#L1183)
  - Adds new rules for `types.KindHealthCheckConfig`

Details for configuring `HealthCheckConfig` with interval, timeout, and healthy/unhealthy thresholds are described in the [database health check RFD](https://github.com/gravitational/teleport/blob/master/rfd/0203-database-healthchecks.md#configuration).


##### Core Kubernetes Agent

The Kubernetes agent registers one or more Kubernetes clusters, checks the health of proxied Kubernetes clusters, and communicates the health state to the auth server. The agent adds a `healthcheck.Manager` which performs the registration and health check operations. The Kubernetes agent is named `TLSServer`, and is located in [lib/kube/proxy/server.go](https://github.com/gravitational/teleport/blob/590c85a765a6d8d16d5f34503179f27a97a4625c/lib/kube/proxy/server.go#L187).

Change points include:

Modifying methods:
- `(*TLSServerConfig).CheckAndSetDefaults()` - Initializes the `healthcheck.Manager`
- `(*TLSServer).Serve()` - Starts `healthcheck.Manager` 
- `(*TLSServer).startStaticClustersHeartbeat()` - Registers all Kubernetes clusters for health monitoring
- `(*TLSServer).close()` - Unregisters all Kubernetes clusters from health monitoring

Adding methods:
- `(*TLSServer).startTargetHealth()` - Registers a single Kubernetes cluster for health monitoring
- `(*TLSServer).stopTargetHealth()` - Unregisters a single Kubernetes cluster from health monitoring
- `(*TLSServer).getTargetHealth()` - Gets health for a single Kubernetes cluster


##### Core `healthcheck` Package

The `healthcheck` package performs recurring health checks on one or more Teleport resources: databases, Kubernetes clusters, etc. It's a general library that currently supports TCP checks. Adding TLS checks is a focus for Kubernetes. 

Main change points are:
- [lib/healthcheck/worker.go](https://github.com/gravitational/teleport/blob/590c85a765a6d8d16d5f34503179f27a97a4625c/lib/healthcheck/worker.go#L343)
  - Modifying the `dialEndpoint()` function to make TLS requests.

[manager.go](https://github.com/gravitational/teleport/blob/590c85a765a6d8d16d5f34503179f27a97a4625c/lib/healthcheck/manager.go) and [target.go](https://github.com/gravitational/teleport/blob/590c85a765a6d8d16d5f34503179f27a97a4625c/lib/healthcheck/target.go) have minor changes.

Prometheus gauge metrics are added to the `healthcheck` package, and described in the [Prometheus Implementation](#prometheus-implementation).


#### Health States

A Kubernetes cluster may be in a health state of `unknown`, `healthy` or `unhealthy`.
- `unknown` indicates a Kubernetes cluster cannot be contacted
- `healthy` indicates a Kubernetes cluster is accepting requests
- `unhealthy` indicates an error state, and includes an error message with verbose debugging information, if available

See the database health check RFD for [more details](https://github.com/gravitational/teleport/blob/master/rfd/0203-database-healthchecks.md#health-status).


##### Health Check Endpoint

> [!NOTE]
> Kubernetes health checked at `https://<address>/readyz`

> [!IMPORTANT]
> Requires Kubernetes v1.16 or higher

The Kubernetes API `/readyz` endpoint is selected for health checking, and indicates that a Kubernetes API server is ready to accept requests. Kubernetes serves the endpoint through [TLS by default](https://kubernetes.io/docs/concepts/security/controlling-access/#transport-security).

The Kubernetes API offers [several health check endpoints](https://kubernetes.io/docs/reference/using-api/health-checks), as well as TCP checks being available.

| Approach        | Description                                              |
|-----------------|----------------------------------------------------------|
| /readyz         | Ready to accept API requests                             |
| /readyz?verbose | Ready to accept API requests (detailed)                  |
| /livez          | kube-apiserver process is alive/running                  |
| /livez?verbose  | kube-apiserver process is alive/running (detailed)       |
| /healthz        | Ambiguously alive or ready. Deprecated in 2019 at v1.16  |
| TCP             | Can establish TCP connection to API server port          |

Let's explore the options and reasoning for selecting `/readyz`.

`/readyz` means that the cluster is accepting API requests, and can be used.

`/livez` indicates the Kubernetes kube-apiserver process is alive. API requests may or may not be accepted. There's no implication of whole cluster readiness.

`/healthz` is deprecated, and not supported with the Kubernetes health check feature. `/healthz` was [deprecated in September of 2019 with Kubernetes v1.16](https://kubernetes.io/blog/2019/09/18/kubernetes-1-16-release-announcement/). At the time of writing, [Kubernetes is at v1.33](https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/), and six years have passed since v1.16. It seems reasonable not to support the `/healthz` endpoint. The choice then sets up a requirement for customers to use Kubernetes v1.16 or higher with Teleport Kubernetes health checks.

Moving on to `TCP`, `TCP` indicates that network connectivity is available. No further knowledge of Kubernetes health would be known. In scenarios where servers don't offer explicit health checks, such as databases, `TCP` may be the only choice. Since Kubernetes offers health checks, we can skip `TCP` checks.

So, `/livez` and `TCP` indicate some level of health, but do not necessarily mean the Kubernetes cluster can be used. 

Let's look at the `verbose` query parameter.

`/readyz?verbose` provides a list of Kubernetes modules with `ok / not ok` states. The verbose information is not critical in the common case of a healthy cluster returning a `200` HTTP status code. The verbose information may be helpful to an administrator diagnosing an unhealthy cluster.

For efficiency in the common case of a healthy cluster, the `/readyz` endpoint is called and checked for a `200` status code. In nearly all cases we only need to check `200`. The `verbose` body message is not sent, reducing unneeded network, memory, and processor consumption. Also, the Kubernetes authors recommend [relying on the status code](https://kubernetes.io/docs/reference/using-api/health-checks/) for checking state.

In the case of non-`200` response codes, a follow-up call to `/readyz?verbose` is made. The follow-up verbose message is appended to a Go error, and eventually forwarded to the Web UI for a Teleport administrator to view.

An example `/readyz?verbose` response body for a `503 Service Unavailable` HTTP status code:
```
[+]ping ok
[+]log ok
[-]etcd not ok: client: etcd cluster is unavailable or misconfigured: context deadline exceeded
[+]poststarthook/start-kube-apiserver-admission-initializer ok
[+]poststarthook/generic-apiserver-start-informers ok
...
[+]shutdown ok
readyz check failed
```

Alternatives not chosen:
- Providing a blend of fallback health checks with `/readyz` -> `/readyz?verbose` -> `/livez?verbose` -> `TCP`. If each call returned a non-`200` error, a fallback approach could be selected. The scenario would capture as much information as is available for the Teleport administrator. The approach is not selected, as it's seen as over-engineering for minimal return.
- Monitoring node and pod health starts to walk into a large universe of Kubernetes observability, which is solved with multiple observability products. It's also worth noting that cluster health is distinct from individual node and pod health. The Kubernetes API server can be healthy and accept requests while individual nodes or pods within the cluster are unhealthy. A cluster may also be in a reduced capacity state where there are a mixture of healthy and unhealthy nodes. These all appear beyond the scope of the RFD. Kubernetes cluster checks provide visibility into resources managed by Teleport, and compliment observability solutions. Various in-depth node and pod metrics are available in observability solutions. The [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics?tab=readme-ov-file#overview) project can be added in [Amazon EKS](https://docs.aws.amazon.com/eks/latest/userguide/metrics-server.html), [Google GKE](https://cloud.google.com/kubernetes-engine/docs/how-to/kube-state-metrics), and [Azure AKS](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-default). Detailed Kubernetes metrics may or may not be interesting for a future feature.
- Checking Kubernetes node health after a cluster is unhealthy may, or may not provide interesting diagnostic information for a Teleport administrator. That could be done via the Kubernetes API. It would add complexity, and may have limited value. Node health could be obtained with an observability solution like kube-state-metrics. Having a downed node doesn't necessarily imply a cluster is unhealthy; perhaps at reduced capacity, or possibly unhealthy. The complexity grows, and may best be addressed by observability solutions.

> [!NOTE]
> Node health != Kubernetes cluster health

> [!NOTE]
> Pod health != Kubernetes cluster health

Calling `/readyz` with a fallback to `/readyz?verbose` achieves the objective of providing the `healthy / unhealthy` state, with diagnostics when needed. Once an unhealthy Kubernetes cluster is detected as unhealthy, a Teleport administrator is expected to follow up with other approaches.


#### `tctl` Implementation

Planned changes to `HealthCheckConfig` percolate to `tctl`. 

No further changes are made for `tctl`.


#### Web UI Implementation

Previous planning and implementation work from database health checks makes displaying Kubernetes health checks straight-forward. No new visual design patterns or coding design patterns are necessary. A Kubernetes health check UI implementation has surgical insertion points.

`TargetHealth` property is added, and `kube_cluster` if/switch case logic is added in approximately nine files: 
- [lib/web/apiserver.go](https://github.com/gravitational/teleport/blob/d08586408b3ad2af327af4af2644f7ac8de4e825/lib/web/apiserver.go#L3414)
- [lib/web/ui/server.go](https://github.com/gravitational/teleport/blob/d08586408b3ad2af327af4af2644f7ac8de4e825/lib/web/ui/server.go#L117)
- [web/packages/shared/components/UnifiedResources/FilterPanel.tsx](https://github.com/gravitational/teleport/blob/d08586408b3ad2af327af4af2644f7ac8de4e825/web/packages/shared/components/UnifiedResources/FilterPanel.tsx#L180)
- [web/packages/shared/components/UnifiedResources/types.ts](https://github.com/gravitational/teleport/blob/d08586408b3ad2af327af4af2644f7ac8de4e825/web/packages/shared/components/UnifiedResources/types.ts#L100)
- [web/packages/shared/components/UnifiedResources/shared/StatusInfo.tsx](https://github.com/gravitational/teleport/blob/d08586408b3ad2af327af4af2644f7ac8de4e825/web/packages/shared/components/UnifiedResources/shared/StatusInfo.tsx#L149)
- [web/packages/shared/components/UnifiedResources/shared/viewItemsFactory.ts](https://github.com/gravitational/teleport/blob/d08586408b3ad2af327af4af2644f7ac8de4e825/web/packages/shared/components/UnifiedResources/shared/viewItemsFactory.ts#L94)
- [web/packages/teleport/src/services/kube/makeKube.ts](https://github.com/gravitational/teleport/blob/d08586408b3ad2af327af4af2644f7ac8de4e825/web/packages/teleport/src/services/kube/makeKube.ts#L21)
- [web/packages/teleport/src/services/kube/types.ts](https://github.com/gravitational/teleport/blob/d08586408b3ad2af327af4af2644f7ac8de4e825/web/packages/teleport/src/services/kube/types.ts#L21)
- [web/packages/teleterm/src/ui/DocumentCluster/UnifiedResources.tsx](https://github.com/gravitational/teleport/blob/d08586408b3ad2af327af4af2644f7ac8de4e825/web/packages/teleterm/src/ui/DocumentCluster/UnifiedResources.tsx#L560)

The Teleport Connect UI is implemented at the same time as the Web UI. Teleport Connect  shares UI components, such as `UnifiedResource`, making the implementation closely related.


#### Prometheus Implementation

Two Prometheus metrics are added to the `healthcheck` package:

- `teleport_health_resources` for the expected number of healthy resources
- `teleport_health_resources_available` for the actual number of healthy resources

The metrics are designed to observe resource health, support multiple resource types (databases, Kubernetes, etc), while keeping the quantity of Prometheus metrics to a minimum. Applying a Prometheus label `type="db|kubernetes|etc"` to a metric enables distinguishing one resource from another.

A difference between `teleport_health_resources` and `teleport_health_resources_available` indicates the presence of unhealthy Kubernetes clusters. A PromQL expression added to a Prometheus alert rule automates detection of unhealthy Kubernetes clusters.
```promql
(teleport_health_resources{type="kubernetes"} - 
 teleport_health_resources_available{type="kubernetes"}) > 0
```

When an unhealthy Kubernetes cluster is detected, a Teleport administrator may use the Teleport Web UI, `tctl` command, or `kubectl` command to identify the Kubernetes cluster, and diagnose further.

Example metric definitions:
```go
  // teleport_health_resources
	resourcesGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace, // teleport
			Subsystem: teleport.MetricHealth,    // health,
			Name:      teleport.MetricResources, // resources
			Help:      "Number of resources being monitored for health.",
		},
		[]string{teleport.TagType}, // db|kubernetes|etc
	)

  // teleport_health_resources_available
	resourcesAvailableGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,          // teleport
			Subsystem: teleport.MetricHealth,             // health
			Name:      teleport.MetricResourcesAvailable, // resources_available
			Help:      "Number of resources in a healthy state.",
		},
		[]string{teleport.TagType}, // db|kubernetes|etc
	)
```

The metric is implemented as a `Gauge` to enable incrementing and decrementing (a counter only increments), and as a `Vec` to enable multiple resource types, `db|kubernetes|etc`.

A resource in a healthy state increments `resourcesAvailableGauge`, and unhealthy and unknown states decrement `resourcesAvailableGauge`.

The metric design in the `healthcheck` package adds Prometheus metrics for Kubernetes health checks as well as database health checks. Future users of the `healthcheck` package, such as MWI, would participate in health metrics.

The design is based on a [proposal by Tim Ross](https://github.com/gravitational/teleport/issues/50285#issuecomment-3198505619). 

Alternatives not chosen:
- Adding metrics only to the Kubernetes agent, and not the `healthcheck` package, would enable Kubernetes-specific metric naming without a `type` label. Metric names `teleport_kubernetes_enrolled` and `teleport_kubernetes_available` may be more evident on first reading. PromQL expressions would be slightly more simple, `teleport_kubernetes_enrolled - teleport_kubernetes_available`. [Existing Teleport Kubernetes metrics](https://github.com/gravitational/teleport/blob/master/docs/pages/includes/metrics.mdx#kubernetes-access) consistently use prefix `teleport_kubernetes_*`. The main drawback of the approach is less reusability across Teleport agents which check health. Each agent would need to write its own health check metrics. The `healthcheck` package is written for reusability across agents. Adding Prometheus metrics to `healthcheck` is a well-factored design at the cost of more verbose metric names, `teleport_health_resources{type="resource"}` and `teleport_health_resources_available{type="resource"}`. 
- Within a `healthcheck` package metrics addition, metric naming might be tailored to an existing naming pattern of `teleport_<resource>_enrolled` / `teleport_<resource>_available`. A pattern of using `type` predicate metrics is also present for [teleport_connected_resources{type="resource"}](https://github.com/gravitational/teleport/blob/bd075a36c19932c285f50b960fdc981aec8436e8/lib/auth/grpcserver.go#L167-L174) and [teleport_reverse_tunnels_connected{type="resource"}](https://github.com/gravitational/teleport/blob/bd075a36c19932c285f50b960fdc981aec8436e8/lib/reversetunnel/localsite.go#L1048-L1054). There are multiple valid approaches used in Teleport. The implementation of a `type` predicate is simpler with a single Prometheus `GaugeVec`. The `teleport_<resource>_enrolled` would use a map to Prometheus `Gauge`, essentially duplicating `GaugeVec`. This is a neutral design choice.

Also see the Prometheus docs for [metric and label naming](https://prometheus.io/docs/practices/naming/) practices, and a [cardinality is key](https://www.robustperception.io/cardinality-is-key/) blog exploring the topic.


### Proxy Routing

In HA deployment scenarios, proxy routing to healthy Kubernetes clusters will be considered. Database health checks provides [proxy routing based healthy database connections](https://github.com/gravitational/teleport/blob/master/rfd/0203-database-healthchecks.md#proxy-behavior). 


### Terraform

The existing `HealthCheckConfig` supports Terraform operations, and will be extended with Kubernetes matchers.


### Documentation

User documentation will be updated with Kubernetes health checks, similar to database health checks.


### Security

Health check calls are made with TLS between Kubernetes agents and proxied Kubernetes clusters.

The existing `health_check_config` configuration is extended, and the existing RBAC security applies. 

Users who are authorized to view `tctl get kube_server` can see health info, which is previously guarded by RBAC.

See the [database health checks RFD](https://github.com/gravitational/teleport/blob/master/rfd/0203-database-healthchecks.md#security) for more details on `health_check_config`. 


### Proto Specification

Several existing protobufs are extended and one new message is added.

Changes focus on adding `TargetHealth` and label matchers. 

**legacy/types/types.proto**
```protobuf
message KubernetesServerV3 {
  // ..snip..

  // Status is the Kubernetes cluster status.
  KubernetesServerStatusV3 Status = 6 [
    (gogoproto.nullable) = false,
    (gogoproto.jsontag) = "status"
  ];
}

// KubernetesServerStatusV3 is the Kubernetes cluster status.
message KubernetesServerStatusV3 {
  // TargetHealth is the health status of network connectivity between
  // the agent and the Kubernetes cluster.
  TargetHealth TargetHealth = 1 [(gogoproto.jsontag) = "target_health,omitempty"];
}
```


**healthcheckconfig/v1/health_check_config.proto**
```protobuf
message Matcher {
  // ..snip..

  // KubernetesLabels matches kubernetes labels. An empty value is ignored. The match
  // result is logically ANDed with KubernetesLabelsExpression, if both are non-empty.
  repeated teleport.label.v1.Label kubernetes_labels = 3;
  // KubernetesLabelsExpression is a label predicate expression to match kubernetes. An
  // empty value is ignored. The match result is logically ANDed with KubernetesLabels,
  // if both are non-empty.
  string kubernetes_labels_expression = 4;
}
```

**lib/teleterm/v1/kube.proto**
```protobuf
message Kube {
  // ..snip..

  // target_health is the health of the kube cluster
  TargetHealth target_health = 4;
}
```

### Backward Compatibility

Kubernetes health checks are backported to `v18`.

The [healthcheck](https://github.com/gravitational/teleport/tree/branch/v18/lib/healthcheck) package used by Kubernetes health checks was introduced in `v18`, and is unsupported in `v16` and `v17`.


### Audit Events

No new audit events. 

Existing `health_check_config` [Create/Update/Delete](https://github.com/gravitational/teleport/blob/master/rfd/0203-database-healthchecks.md#audit-events) events are exercised.


### Observability

Two Prometheus metrics are implemented in the `healthcheck` package, and described in the [Prometheus Implementation](#prometheus-implementation).

Log messages are emitted when helpful.


### Test Plan

A Kubernetes health check test plan closely mirrors the database health check plan.

The following steps are added:

```markdown
### Kubernetes Health Checks
  - [ ] Verify health checks with `tctl`
    - [ ] `tctl get kube_server` includes `kube_server.status.target_health` info
    - [ ] `tctl update health_check_config` resets `kube_server.status.target_health.status` with matching Kubernetes clusters. This may take several minutes.
    - [ ] Disabling health checks shows `kube_server.status.target_health` as "unknown/disabled". This may take several minutes. There are a couple ways to achieve this.
        - [ ] `tctl update health_check_config` 
        - [ ] `tctl delete health_check_config`
  - [ ] Verify health checks with the web UI
    - [ ] Configure a Kubernetes agent with a Kubernetes cluster with an unreachable endpoint.
    - [ ] The web UI resource page shows a warning indicator for that Kubernetes cluster with error details.
    - [ ] Without restarting the Kubernetes agent, make the Kubernetes cluster endpoint reachable. Observe that the warning indicator disappears after some time.
```

### Implementation Phases

#### Phase 1: Core Health Checks

Integrate health checks into the Kubernetes agent and proxy. 

Health checks are performed from the Teleport Kubernetes agent and queries Kubernetes clusters.

Health checks are reported to the Teleport auth server.

Kubernetes health checks are configured and viewable from `tctl`.

#### Phase 2: Web/Teleterm UI Health Checks

Kubernetes health checks are displayed and updated in the Web UI.

Kubernetes health checks are displayed and updated in the Teleterm UI.

#### Phase 3: Prometheus Health Checks

Kubernetes health checks are viewable from Prometheus metrics.

#### Phase 4: Documentation

Add user documentation.