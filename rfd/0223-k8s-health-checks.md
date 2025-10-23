---
authors: Rana Ian (rana.ian@goteleport.com)
state: draft
---

# RFD 0223 - Kubernetes Health Checks

## Required Approvals

- Engineering: @rosstimothy && (@creack || @tigrato || @GavinFrazar)


## What

Enable automated Kubernetes cluster health checks that are viewed by the Web UI, Teleport Connect, `tctl` command, and Prometheus metrics.

## Why

Proxied Kubernetes cluster health may be manually exercised with Kubernetes operations in the Web UI, or `kubectl` command, and observing the result. While effective, manual checking is also slow and unscalable. 

Automated Kubernetes health checks with new methods of viewing improve maintainability of Teleport Kubernetes clusters, while enabling new scenarios.

Automated health checks:
- Improve time to observe and resolve Kubernetes cluster errors
- Improve manageability of Kubernetes clusters, especially at scale
- Improve the enrollment experience for Kubernetes clusters
- Enable alerting on unhealthy Kubernetes clusters
- Provide feature parity with databases

## Details

### UX

#### User Story: Web UI - Enrolling Kubernetes Clusters

Alice enrolls three Amazon EKS clusters into Teleport through the Web UI.

The next day she returns to the Web UI _Resources_ tab to find Amazon EKS clusters are highlighted with warnings. She clicks on an EKS tile and a health message is displayed in a side panel.

```
Kubernetes Cluster Issues

3 Teleport Kubernetes clusters report issues.

Affected Teleport Kubernetes cluster:
- Hostname: sol
  UUID: 52dedbd0-b165-4bf6-9bc3-961f95bf481d
  Error: Unable to retrieve pods from the Kubernetes cluster. Please see the Kubernetes Access Troubleshooting guide, https://goteleport.com/docs/enroll-resources/kubernetes-access/troubleshooting/.

Affected Teleport Kubernetes cluster:
- Hostname: jupiter
  UUID: bb4dc171-ffa7-4a31-ba8c-7bf91c59e250
  Error: Unable to retrieve pods from the Kubernetes cluster. Please see the Kubernetes Access Troubleshooting guide, https://goteleport.com/docs/enroll-resources/kubernetes-access/troubleshooting/.

Affected Teleport Kubernetes cluster:
- Hostname: saturn
  UUID: 2be08cb1-56a4-401f-a3f3-c755a73f3ff6
  Error: Unable to retrieve pods from the Kubernetes cluster. Please see the Kubernetes Access Troubleshooting guide, https://goteleport.com/docs/enroll-resources/kubernetes-access/troubleshooting/.
```

Alice notices that each cluster has similar `access denied` errors.

Alice applies new Kubernetes RBAC, and clusters return to a healthy state.

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

He exercises the Kubernetes health checks in non-production environments.

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

He enrolls three GKE instances to Teleport.

He runs Prometheus query expressions to check the new health metrics.
```promql
teleport_resources_health_status_unhealthy{type="kubernetes"}
# Returns 0, the number of unhealthy Kubernetes clusters

teleport_resources_health_status_healthy{type="kubernetes"} +
teleport_resources_health_status_unhealthy{type="kubernetes"} +
teleport_resources_health_status_unknown{type="kubernetes"}
# Returns 3, the total number of Kubernetes clusters
```

Charlie sets one GKE instance into an unhealthy state and requeries.

```promql
teleport_resources_health_status_unhealthy{type="kubernetes"}
# Returns 1, the number of unhealthy Kubernetes clusters

teleport_resources_health_status_healthy{type="kubernetes"} +
teleport_resources_health_status_unhealthy{type="kubernetes"} +
teleport_resources_health_status_unknown{type="kubernetes"}
# Returns 3, the total number of Kubernetes clusters
```

Seeing metric values returning, he sets up a Prometheus alerting rule.

```yaml
groups:
  - name: teleport_kubernetes
    rules:
      - alert: KubernetesClusterUnhealthy
        expr: |
          teleport_resources_health_status_unhealthy{type="kubernetes"} > 0
        for: 5m
        labels:
          severity: warning
          team: platform
          component: teleport
          service: kubernetes
        annotations:
          summary: "{{ $value }} Kubernetes cluster(s) unhealthy in Teleport"
          description: "Teleport reports {{ $value }} unhealthy Kubernetes cluster(s). Kubernetes clusters registered with Teleport are failing health checks. Check Teleport Web UI or use tctl get kube_server for details."
          query: teleport_resources_health_status_unhealthy{type="kubernetes"} > 0
```

Prometheus alerts him about the unhealthy Kubernetes cluster.

Charlie sets the GKE instance into a healthy state and moves on with his day.


### Implementation Details

Kubernetes health checks are discussed by functional areas of core logic, `tctl` command, Web UI, and Prometheus metrics.

#### Core Implementation

Teleport Kubernetes health checks use the Teleport `healthcheck` package, and is based on existing [database health check](https://github.com/gravitational/teleport/blob/master/rfd/0203-database-healthchecks.md#user-content-fn-1-b6df2ad8fd7a63ee3ca0af227e74ab87) design patterns. The `healthcheck` components are written, tested, and in production. The focus and effort for Kubernetes is integrating health checks into the Kubernetes agent, extending existing `healthcheck` mechanisms, and updating the UI.

##### Core Configuration

A first step to enabling Kubernetes health checks is adding new matchers to the `HealthCheckConfig` service. `HealthCheckConfig` identifies servers which choose to participate in health checking.

Matchers `kubernetes_labels` and `kubernetes_labels_expression` are added to specify labeled Kubernetes clusters. By default, the preset setting is defined to enable all Kubernetes clusters to participate in health checks. Manually specifying matchers may filter out specific Kubernetes clusters. Editing `HealthCheckConfig` to omit Kubernetes matchers would exclude all Kubernetes clusters from health checks.

An example `health_check_config`:
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
    db_labels:
      - name: "*"
        values:
          - "*"
    db_labels_expression: "labels.env != `prod`"
```

`HealthCheckConfig` may be communicated via proxy and is cached on a Kubernetes agent. Kubernetes Go interfaces are updated to support the proxy communication and caching. Pre-existing mechanisms for configuring `HealthCheckConfig` with interval, timeout, and healthy/unhealthy thresholds are described in the [database health check RFD](https://github.com/gravitational/teleport/blob/master/rfd/0203-database-healthchecks.md#configuration).


##### Core Kubernetes Agent

The Kubernetes agent registers one or more Kubernetes clusters, checks the health of proxied Kubernetes clusters, and communicates a health state back to the auth server. The agent adds a `healthcheck.Manager`, which registers Kubernetes clusters and schedules health checks. 


##### Core `healthcheck` Package

The `healthcheck` package performs recurring health checks on one or more Teleport resources: databases, Kubernetes clusters, etc. It's a general library that currently supports TCP checks. Extending support to Kubernetes API calls is a focus. 

The [healthcheck.Target](https://github.com/gravitational/teleport/blob/dcd4a9b1f88d76bc7d6617083e539cc6c324dc3a/lib/healthcheck/target.go#L37) adds a `CheckHealth` function field, and removes a `ResolverFn` function field. The existing logic using `ResolverFn` for database health checks is encapsulated into a new database-specific `CheckHealth` function then passed to the `Target.CheckHealth` function field.

```diff
// Target is a health check target.
type Target struct {
	// GetResource gets a copy of the target resource with updated labels.
	GetResource func() types.ResourceWithLabels
-	// ResolverFn resolves the target endpoint(s).
-	ResolverFn EndpointsResolverFunc
+	// Checks the health of a target resource.
+	CheckHealth func(ctx context.Context) error
}
```

A `healthcheck` [worker](https://github.com/gravitational/teleport/blob/dcd4a9b1f88d76bc7d6617083e539cc6c324dc3a/lib/healthcheck/worker.go#L278) calls the new `CheckHealth` function.

Prometheus gauge metrics are also added to the `healthcheck` package, and described in the [Prometheus Implementation](#prometheus-implementation).


#### Health Checks

Kubernetes cluster health is detected by calling Kubernetes API [SelfSubjectAccessReview](https://kubernetes.io/docs/reference/access-authn-authz/authorization/#checking-api-access) endpoints through TLS.

Four API calls are made per health check.

Endpoint `/apis/authorization.k8s.io/v1/selfsubjectaccessreviews`:
- verb: impersonate, resource: users
- verb: impersonate, resource: groups
- verb: impersonate, resource: serviceaccounts
- verb: get, resource: pods

The API calls exercise Teleport Kubernetes RBAC. Positive responses indicate the Kubernetes cluster is properly configured with a [Teleport ClusterRole](https://github.com/gravitational/teleport/blob/master/examples/chart/teleport-kube-agent/templates/clusterrole.yaml) and responds to requests. 

A healthy Kubernetes cluster is defined as a customer being able to use a Kubernetes cluster. Exercising the `/selfsubjectaccessreviews` endpoint enables checking RBAC in addition to other layers of Kubernetes functionality. It addresses usability from a customer's point of view.

TCP and other Kubernetes health endpoints `readyz` / `livez` / `healthz` were explored. Each indicates a level of Kubernetes cluster health, none ensures that a customer can actually use the cluster.



##### Health Check Alternatives Which Were Considered

Kubernetes offers an API with [several health check endpoints](https://kubernetes.io/docs/reference/using-api/health-checks), as well as TCP checks being available.

| Approach        | Description                                              |
|-----------------|----------------------------------------------------------|
| /readyz         | Ready to accept API requests                             |
| /readyz?verbose | Ready to accept API requests (detailed)                  |
| /livez          | kube-apiserver process is alive/running                  |
| /livez?verbose  | kube-apiserver process is alive/running (detailed)       |
| /healthz        | Ambiguously alive or ready. Deprecated in 2019 at v1.16  |
| TCP             | Can establish TCP connection to API server port          |

Let's explore the options.

`/readyz` means that the cluster is accepting API requests, and can be used.

`/livez` indicates the Kubernetes kube-apiserver process is alive. API requests may or may not be accepted. There's no implication of whole cluster readiness.

`/healthz` is deprecated, and not supported with the Kubernetes health check feature. `/healthz` was [deprecated in September of 2019 with Kubernetes v1.16](https://kubernetes.io/blog/2019/09/18/kubernetes-1-16-release-announcement/). At the time of writing, [Kubernetes is at v1.33](https://kubernetes.io/blog/2025/04/23/kubernetes-v1-33-release/), and six years have passed since v1.16. It seems reasonable not to support the `/healthz` endpoint. That choice would then set up a requirement for customers to use Kubernetes v1.16 or higher with Teleport Kubernetes health checks.

Moving on to `TCP`, `TCP` indicates that network connectivity is available. No further knowledge of Kubernetes health would be known. In scenarios where servers don't offer explicit health checks, such as databases, `TCP` may be the only choice. Since Kubernetes offers health checks, we can skip `TCP` checks.

So, `/livez` and `TCP` indicate some level of health, but do not necessarily mean the Kubernetes cluster can be used. 

Let's look at the `verbose` query parameter.

`/readyz?verbose` provides a list of Kubernetes modules with `ok / not ok` states. The verbose information is not critical in the common case of a healthy cluster returning a `200` HTTP status code. The verbose information may be helpful to an administrator diagnosing an unhealthy cluster.

For efficiency in the common case of a healthy cluster, the `/readyz` endpoint could be called and checked for a `200` status code. In nearly all cases we would only need to check `200`, and the `verbose` body message would not be sent, reducing unneeded network, memory, and processor consumption. Also, the Kubernetes authors recommend [relying on the status code](https://kubernetes.io/docs/reference/using-api/health-checks/) for checking state.

In the case of non-`200` response codes, a follow-up call to `/readyz?verbose` could be made. The follow-up verbose message may be appended to a Go error, and eventually forwarded to the Web UI for a Teleport administrator to view.

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
Calling `/readyz` with a fallback to `/readyz?verbose` provides the `healthy / unhealthy` states, with diagnostics when needed. It would not necessarily indicate that a customer has properly configured Teleport Kubernetes RBAC. And would not indicate that the customer can actually use a Kubernetes cluster.

Further alternatives considered:
- Providing a blend of fallback health checks with `/readyz` -> `/readyz?verbose` -> `/livez?verbose` -> `TCP`. If each call returned a non-`200` error, a fallback approach could be selected. The scenario would capture as much information as is available for the Teleport administrator. The approach is not selected, as it's seen as over-engineering for minimal return.
- Monitoring node and pod health starts to walk into a large universe of Kubernetes observability, which is solved with multiple observability products. It's also worth noting that cluster health is distinct from individual node and pod health. The Kubernetes API server can be healthy and accept requests while individual nodes or pods within the cluster are unhealthy. A cluster may also be in a reduced capacity state where there are a mixture of healthy and unhealthy nodes. These all appear beyond the scope of the RFD. Kubernetes cluster checks provide visibility into resources managed by Teleport, and compliment observability solutions. Various in-depth node and pod metrics are available in observability solutions. The [kube-state-metrics](https://github.com/kubernetes/kube-state-metrics?tab=readme-ov-file#overview) project can be added in [Amazon EKS](https://docs.aws.amazon.com/eks/latest/userguide/metrics-server.html), [Google GKE](https://cloud.google.com/kubernetes-engine/docs/how-to/kube-state-metrics), and [Azure AKS](https://learn.microsoft.com/en-us/azure/azure-monitor/containers/prometheus-metrics-scrape-default). Detailed Kubernetes metrics may or may not be interesting for a future feature.
- Checking Kubernetes node health after a cluster is unhealthy may, or may not provide interesting diagnostic information for a Teleport administrator. That could be done via the Kubernetes API. It would add complexity, and may have limited value. Node health could be obtained with an observability solution like `kube-state-metrics`. Having a downed node doesn't necessarily imply a cluster is unhealthy; perhaps at reduced capacity, or possibly non-functional. The complexity grows, and may best be addressed by observability solutions.

> [!NOTE]
> Node health != Kubernetes cluster health

> [!NOTE]
> Pod health != Kubernetes cluster health


#### `tctl` Implementation

Planned changes to `HealthCheckConfig` percolate to `tctl`. 

No further changes are made for `tctl`.


#### Web UI Implementation

Previous planning and implementation work from database health checks makes displaying Kubernetes health checks straight-forward. No new visual design patterns or coding design patterns are necessary. A Kubernetes health check UI implementation has surgical insertion points.

`TargetHealth` property and `kube_cluster` if/switch logic is added in approximately nine files.

The Teleport Connect UI is implemented at the same time as the Web UI. Teleport Connect shares UI components, such as `UnifiedResource`, making the implementation closely related.

User friendly error messages are displayed with a link to the Kubernetes Access Troubleshooting guide. The guide will be updated with each error message and resolution steps.

#### Health States

A Kubernetes cluster may be in a `healthy`, `unhealthy`, or `unknown` state.
- `healthy` indicates a Kubernetes cluster may be used by a customer
- `unhealthy` indicates a Kubernetes cluster is an error state, and includes an error message
- `unknown` indicates a Kubernetes cluster cannot be contacted

See the database health check RFD for [more details](https://github.com/gravitational/teleport/blob/master/rfd/0203-database-healthchecks.md#health-status).


#### Prometheus Implementation

Three Prometheus metrics are added to the `healthcheck` package:

- `teleport_resources_health_status_healthy` for the number of healthy resources
- `teleport_resources_health_status_unhealthy` for the number of unhealthy resources
- `teleport_resources_health_status_unknown` for the number of resources in an unknown health state

The metrics are designed to observe resource health, support multiple resource types (databases, Kubernetes, etc), while keeping the quantity of Prometheus metrics to a minimum. Applying a Prometheus label `type="db|kubernetes|etc"` to a metric distinguishes one resource from another.

Use a PromQL expression to determine the total number of Kubernetes clusters.
```promql
teleport_resources_health_status_healthy{type="kubernetes"} +
teleport_resources_health_status_unhealthy{type="kubernetes"} +
teleport_resources_health_status_unknown{type="kubernetes"}
```

Use a PromQL expression to detect the presence of unhealthy Kubernetes clusters.
```promql
teleport_resources_health_status_unhealthy{type="kubernetes"} > 0
```

When an unhealthy Kubernetes cluster is detected, a Teleport administrator may use the Teleport Web UI, `tctl` command, or `kubectl` command to identify the Kubernetes cluster, and diagnose further.


The metrics are implemented as gauges to enable incrementing and decrementing (a counter only increments), and as a `Vec` to enable multiple resource types, `db|kubernetes|etc`.

Here are example metric definitions:
```go
	// teleport_resources_health_status_healthy
	resourceHealthyGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: teleport.MetricResourcesHealthStatus,
			Name:      teleport.MetricHealthy,
			Help:      "Number of healthy resources",
		},
		[]string{teleport.TagType}, // db|k8s|etc
	)
	// teleport_resources_health_status_unhealthy
	resourceUnhealthyGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: teleport.MetricResourcesHealthStatus,
			Name:      teleport.MetricUnhealthy,
			Help:      "Number of unhealthy resources",
		},
		[]string{teleport.TagType}, // db|k8s|etc
	)
	// teleport_resources_health_status_unknown
	resourceUnknownGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: teleport.MetricNamespace,
			Subsystem: teleport.MetricResourcesHealthStatus,
			Name:      teleport.MetricUnknown,
			Help:      "Number of resources in an unknown health state",
		},
		[]string{teleport.TagType}, // db|k8s|etc
	)
```

Health check metrics are incremented and decremented in `worker.go` during state changes, and decremented on closing. Each metric is labeled with a resource type such as `db` or `kubernetes`.

Database health checks, which use the existing `healthcheck` package, now emit health metrics. Future components using the `healthcheck` package, such as MWI, would also emit health metrics.


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

The documentation will point out that Prometheus metrics only track configured health checks. It's possible for Kubernetes clusters to exist, and be skipped in health check monitoring, and not visible from Prometheus.

Kubernetes health check documentation is modeled on the existing [Database Health Checks documentation](https://goteleport.com/docs/enroll-resources/database-access/guides/health-checks/).

The [Kubernetes Access Troubleshooting](https://goteleport.com/docs/enroll-resources/kubernetes-access/troubleshooting/) guide will be updated with user-friendly error returned by health checks, and related  resolution steps.

### Security

Health check calls are made with TLS between Kubernetes agents and proxied Kubernetes clusters.

The existing `health_check_config` configuration is extended, and the existing RBAC security applies. 

Users who are authorized to view `tctl get kube_server` can see health info, which is previously guarded by RBAC.

See the [database health checks RFD](https://github.com/gravitational/teleport/blob/master/rfd/0203-database-healthchecks.md#security) for more details on `health_check_config`. 


### Proto Specification

Several existing protobufs are extended and one new message is added.

Changes focus on adding `TargetHealth` and label matchers.

Definitions apply modern protobuf naming conventions, and omit depreciated `gogoproto` tags.

**legacy/types/types.proto**
```diff
// KubernetesServerV3 represents a Kubernetes server.
message KubernetesServerV3 {
  option (gogoproto.goproto_stringer) = false;
  option (gogoproto.stringer) = false;

  // Kind is the Kubernetes server resource kind. Always "kube_server".
  string Kind = 1 [(gogoproto.jsontag) = "kind"];
  // SubKind is an optional resource subkind.
  string SubKind = 2 [(gogoproto.jsontag) = "sub_kind,omitempty"];
  // Version is the resource version.
  string Version = 3 [(gogoproto.jsontag) = "version"];
  // Metadata is the Kubernetes server metadata.
  Metadata Metadata = 4 [
    (gogoproto.nullable) = false,
    (gogoproto.jsontag) = "metadata"
  ];
  // Spec is the Kubernetes server spec.
  KubernetesServerSpecV3 Spec = 5 [
    (gogoproto.nullable) = false,
    (gogoproto.jsontag) = "spec"
  ];
+  // Status is the Kubernetes server status.
+  KubernetesServerStatusV3 status = 6;
}

+// KubernetesServerStatusV3 is the Kubernetes cluster status.
+message KubernetesServerStatusV3 {
+  // TargetHealth is the health status of network connectivity between
+  // the agent and the Kubernetes cluster.
+  TargetHealth target_health = 1;
+}
```


**healthcheckconfig/v1/health_check_config.proto**
```diff
// Matcher is a resource matcher for health check config.
message Matcher {
  // DBLabels matches database labels. An empty value is ignored. The match
  // result is logically ANDed with DBLabelsExpression, if both are non-empty.
  repeated teleport.label.v1.Label db_labels = 1;
  // DBLabelsExpression is a label predicate expression to match databases. An
  // empty value is ignored. The match result is logically ANDed with DBLabels,
  // if both are non-empty.
  string db_labels_expression = 2;
+ // KubernetesLabels matches kubernetes labels. An empty value is ignored. The match
+ // result is logically ANDed with KubernetesLabelsExpression, if both are non-empty.
+ repeated teleport.label.v1.Label kubernetes_labels = 3;
+ // KubernetesLabelsExpression is a label predicate expression to match kubernetes. An
+ // empty value is ignored. The match result is logically ANDed with KubernetesLabels,
+ // if both are non-empty.
+ string kubernetes_labels_expression = 4;
}
```

**lib/teleterm/v1/kube.proto**
```diff
// Kube describes connected Kubernetes cluster
message Kube {
  // uri is the kube resource URI
  string uri = 1;
  // name is the kube name
  string name = 2;
  // labels is the kube labels
  repeated Label labels = 3;
+ // target_health is the health of the kube cluster
+ TargetHealth target_health = 4;
}
```

### Backward Compatibility

Kubernetes health checks are backported to `v18`.

The [healthcheck](https://github.com/gravitational/teleport/tree/branch/v18/lib/healthcheck) package used by Kubernetes health checks was introduced in `v18`, and is unsupported in `v16` and `v17`.

#### Adding Kubernetes Label Matchers Verified as Backward Compatible

Backward compatibility for adding Kubernetes label matchers to `v18` was tested and verified to function properly.

Testing was performed on a development machine.

Teleport `v19` proof-of-concept was run with auth+proxy+kube. The health check config was edited to include only Kubernetes label matchers. Storage of the edited health check config to the backend `events` table was double checked by viewing `/health_check_config/default` key data with [DB Browser for SQLite](https://sqlitebrowser.org/). Only the Kubernetes wildcard matchers were present (no db matchers present).

Teleport `v18` auth+proxy+kube was run with the identical configuration and backend database. `v18` runs without issue. Kubernetes health checks are simply omitted on a `v18` without backporting.

Validation of health check config is performed only on writes to the backend database with [ValidateHealthCheckConfig](https://github.com/gravitational/teleport/blob/447cb50d24b763b9a3a6c78558560ba05c89ef18/lib/services/health_check_config.go#L58), and not during reads of the config.

Customers would update to `v19` or a `v18` with a backport to participate in Kubernetes health checks.


### Audit Events

No new audit events. 

Existing `health_check_config` [Create/Update/Delete](https://github.com/gravitational/teleport/blob/master/rfd/0203-database-healthchecks.md#audit-events) events are exercised.


### Observability

Three Prometheus metrics are implemented in the `healthcheck` package, and described in the [Prometheus Implementation](#prometheus-implementation).

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
  - [ ] Exercise agents/proxies running an older version of Teleport in a mixed fleet scenario.
    - [ ] Exercise only Kubernetes matchers (no db matchers)
    - [ ] Exercise Kubernetes matchers and database matchers
    - [ ] Exercise zero matchers 
```

### Implementation Phases

Implementation starts with foundational elements in core health checks, and continues to UI and documentation.

#### Phase 1: Core Health Checks

##### PR 1: Prometheus Health Checks

Prometheus metrics are added to the `healthcheck` package. This is straight-forward addition with few dependencies.

##### PR 2: Health Check Protobufs

Protobufs form a foundation and are well-defined.

#### Further PRs

Integrate health checks into the Kubernetes agent and proxy. 

Health checks are performed from the Teleport Kubernetes agent and makes calls to Kubernetes clusters.

Health checks are reported to the Teleport auth server.

Kubernetes health checks are configured and viewable from `tctl`.


#### Phase 2: UI Health Checks

Kubernetes health checks are displayed and updated in the Web UI.

Kubernetes health checks are displayed and updated in the Teleport Connect UI.


#### Phase 3: Documentation

Add user documentation.
