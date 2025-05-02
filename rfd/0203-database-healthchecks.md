---
authors: Gavin Frazar (gavin.frazar@goteleport.com)
state: draft
---

# RFD 0203 - Database Health Checks

## Required Approvals

- Engineering: @r0mant && @greedy52
- Product: @roraback

## What

Improve availability of databases proxied by more than one agent (i.e., a high-availability setup [^1]), such that users do not experience random connection failures when only a subset of agents can reach the database over the network.

Network health between an agent and the database it proxies will also be made visible to users to surface network problems and help the user troubleshoot.

During resource onboarding, the network health of a newly registered database will be used to automatically detect misconfiguration and provide troubleshooting tips and suggestions.

[^1]: https://goteleport.com/docs/enroll-resources/database-access/guides/ha/#combined-replicas

## Why

1. Fix random user connection failures caused by network issues in HA agent deployments. This issue is a frequent source of support tickets and user frustration.
2. Improve database resource onboarding UX
3. Surface agent->resource connectivity issues early
4. Improve troubleshooting
5. Lay the groundwork for collecting agent->resource latency measurements that can be used to make routing decisions based on latency

Related issues:
- [Database Access upstream DB health checks](https://github.com/gravitational/teleport/issues/20544)
- [Desktop Access: support latency-based routing to a desktop service](https://github.com/gravitational/teleport/issues/40905)
- [Detect and report latency measurements for Desktop sessions in the UI](https://github.com/gravitational/teleport/issues/35691)

## Details

Teleport database agents will periodically perform health checks to a database's endpoint and report the results as a health status in the agent's `db_server` heartbeat.

When the Teleport proxy routes a user connection to a database agent it will prioritize database agents that have healthy network connectivity to the database endpoint over agents that do not.

Currently, in an HA DB agent configuration, the proxy will randomly choose one of the DB agents to handle a user connection.

If a subset of the DB agents can't reach the database endpoint, then user connection attempts will randomly fail.

To fix this, the Teleport proxy service will group DB agents by target health status, shuffle each group, and then preferentially dial agents that have healthy network connectivity to the database.

### UX

The web UI will be updated to leverage database health status information.

Unhealthy databases will be displayed with a warning icon indicating that the database is unhealthy.

Recall that there may be multiple `db_server` heartbeats for the same database, but the web UI only shows a single resource tile for the database.
If this is the case, then the warning icon will be shown if any of the heartbeats have an unhealthy `target_health.status`.

Clicking on the warning icon will open a side panel with more information.
The side panel content may look something like this (or equivalent singular form verbiage if there's only one agent):

    M out of N Teleport database services proxying access to this database cannot reach the database endpoint.
    <link to healthcheck/network troubleshooting doc>

    Affected Teleport database services:
    - Hostname: <hostname>
      UUID: <uuid>
      Error: <last health check error message>
    - Hostname: <hostname>
      UUID: <uuid>
      Error: <last health check error message>

Databases with a mix of `healthy` and `""` or `unknown` status will also show a warning icon.
The side panel will encourage the user to enable health checks or update to a supported version on all of the agents that proxy the db.
The side panel content may look something like this: 

    M out of N Teleport database services proxying access to this database are not running network health checks for the database endpoint.
    User connections will not be routed through affected Teleport database services as long as other database services report a healthy connection to the database.

    Affected Teleport database services:
    - Hostname: <hostname>
      UUID: <uuid>
      Reason: The database service version v17.1.2 does not support health checks.
    - Hostname: <hostname>
      UUID: <uuid>
      Reason: The database service has disabled health checks for this database.

#### User story: updating health check configuration

Alice is a Teleport cluster admin who recently updated her cluster to a Teleport major version that supports health checks.

After reading the changelog and supporting reference documentation for health checks, Alice checks the default settings using tctl:

    $ tctl get health_check_config/default
    kind: health_check_config
    version: v1
    metadata:
      name: "default"
      labels:
        teleport.internal/resource-type: preset
    spec:
      match:
        db_labels:
          - name: '*'
            values:
            - '*'
    
Out of an abundance of caution around this new feature, Alice wants to opt-out of health checks for prod databases.

Alice uses `tctl edit health_check_config/default` to update the default settings and exclude prod databases:

    kind: health_check_config
    version: v1
    metadata:
      name: "default"
      labels:
        teleport.internal/resource-type: preset
    spec:
      match:
        db_labels:
          - name: '*'
            values:
            - '*'
        db_labels_expression: "labels.env != `prod`"

#### User story: AWS RDS database enrollment (Phase 2)

A Teleport cluster admin is new to Teleport and would like to enroll an AWS RDS database with Teleport.

- The user logs into Teleport, sees the "Enroll New Resource" button and clicks that.
- They click on the "RDS PostgreSQL" tile - a guided database enrollment wizard.
- After selecting an AWS integration, region, and VPC, the user sees the "Enroll RDS Database" page.
- The user selects a database to enroll, and clicks "Next". 
- On the next page, the user is asked to configure an IAM role, choose AWS subnets and security groups, and finally deploy a Teleport database service in ECS using those settings.
- The user selects a public subnet and a security group that allows all outbound traffic, then clicks "Deploy".

The page tells the user that it is waiting for the new DB agent to start and join the Teleport cluster:

    Teleport is currently deploying a Database Service.
    It will take at least a minute for the Database Service to be created and joined to your cluster.

After the new agent joins, the page shows a success message:

    Successfully created and detected your new Database Service.

The page displays a new message below that:

    Teleport is testing network connectivity between the Database Service and your database.

After a short time, the page displays a message telling the user that the agent cannot reach the database endpoint:

    The Database Service does not have connectivity to the database.
    1. Check that the security groups you selected allow outbound TCP traffic from the Teleport Database Service to the database address on port 5432
    2. Check that the database security groups allow inbound traffic from the Teleport Database Service on port 5432

    Troubleshooting tips: <link to teleport docs>

- The user realizes that they need to select an additional security group that the database allows inbounds traffic from.
- They select the additional security group and click "Redeploy".

The page displays the same messages waiting for the deployment to join the Teleport cluster, and then displays this message again: 

    Teleport is testing network connectivity between the Database Service and your Database.

After a few seconds, the page displays a new message:

    The Database Service has established network connectivity with the your Database!

The user is now allowed to proceed with the remaining steps in the enrollment flow.

> [!NOTE]
> As of writing, one of the post-deployment steps in the enrollment wizard is a "connection tester", which tests a combination of network connectivity, Teleport RBAC, AWS IAM permissions, and RDS IAM config in the database itself.
> The problem is that if the connection tester finds a network connectivity issue, then the user would have to go back to redeploy the database agent to fix it.
> Network health checks can be used to ensure that the deployment's network settings are correct before the user moves on from the deployment step.
> The other failure modes do not depend on the deployment settings, so it makes sense to keep those tests in a separate subsequent step.

#### User story: web UI resource health indicators

Alice has deployed a Teleport database agent and enrolled a PostgreSQL database with health checks enabled and using the default settings.

Some time later, another user reconfigures the database firewall rules to restrict the allowed inbound traffic IP ranges, but they inadvertently block the Teleport database agent from reaching the database over the network.

Alice logs into the web UI and navigates to the resources page. 

Alice sees a warning icon on the database she set up earlier.

Alice clicks on the icon and a side panel pops out:

    The Teleport database service proxying access to this database cannot reach the database endpoint.
    <link to health check or network troubleshooting doc>

    Affected Teleport database service:
    - Hostname: <hostname>
      UUID: <uuid>
      Error: "(tcp) failed: Operation timed out"

Among other suggestions, the documentation includes:
- check that the database is actually listening on `<port>`
- check that database inbound TCP traffic is allowed from the agent to `<ip : port>`
- check that agent outbound TCP traffic is allowed to `<ip : port>`

After checking the linked documentation and some investigation Alice diagnoses that the timeout is caused by the database's firewall dropping packets from the agent's IP, which she resolves by updating the firewall rules.

After a short (<20s) time the threshold is reached and the agent changes its status to `healthy`.

The warning on the resource tile goes away.

Alice checks the health status manually with tctl:

```
$ tctl get db_server/example

kind: db_server
metadata:
  expires: "2025-02-21T02:03:47.398926Z"
  name: example
  revision: 43e96231-faaf-43c3-b9b8-15cf91813389
spec:
  ...*snip*...
  host_id: 278be63c-c87e-4d7e-a286-86002c7c45c3
  hostname: mars.internal
  ...*snip*...
status:
  target_health:
    addr: example.com:5432
    protocol: TCP
    transition_timestamp: "2025-02-19T01:53:39.144218Z"
    transition_reason: "healthy threshold reached"
    status: healthy
  version: 18.0.0-dev
version: v3
```

#### Out of scope

The following ideas were considered, but ultimately cut from the initial implementation of this RFD to reduce its scope:

1. Add a new type of notification routing rule that the auth service can use to send a notification when a target becomes unhealthy, much like the `access_request_routing_rules` added in [RFD 87 - Access request notification routing](./0087-access-request-notification-routing.md).
2. Automatically create a "user task" in the AWS integration dashboard when an AWS RDS database's health status is unhealthy.

These are both good improvements for monitoring and may be added to this RFD in future work.

### Health status

Database target health status will be stored in the DB agent's ephemeral `db_server` heartbeat as the `status.target_health` field.
The `target_health` will include, among other things, a `status` field.

These are the possible values for the `db_server.status.target_health.status` field:
- `""`
- `unknown`
- `healthy`
- `unhealthy`

An empty status `""` indicates an unknown status.
If an older agent is proxying the DB, then the health status will be empty.

The `unknown` status indicates that health checks are disabled, still initializing, or unsupported for this database type.

The `healthy` status is reported after the number of consecutive passing health checks has reached a healthy threshold.

The `unhealthy` status is reported after the number of consecutive failing health checks has reached an unhealthy threshold.

As a special case the first health check will change the status to `healthy` or `unhealthy` regardless of the configured thresholds.
This special case is to bound the amount of time spent in the `unknown` status if health checks flap between pass/fail without reaching either threshold.

### Types of health checks

For the initial implementation we will only support TCP health checks and only for databases.

The initial implementation will support health checks for Postgres, MySQL, and MongoDB databases.
We will gradually expand support to other databases.

We can extend this feature later to include additional health check protocols such as HTTP and TLS checks.

We can also extend this feature to other one-agent-to-many-resources types of resources: apps, Windows desktops, OpenSSH servers, etc.

#### TCP health check

A TCP health check attempts to establish a TCP connection to the target address.
If dialing the target address times out or receives a TCP reset (RST), then the check has failed.
Otherwise, the check has passed.

### Configuration

Health checks will be an opt-out configurable setting with reasonable defaults.

A new cluster resource will be added to manage health check settings: `health_check_config`.

A `health_check_config` preset resource named "default" will be added.

The "default" `health_check_config` preset will enable health checks for all databases.

Users will be able to easily opt-out of health checks by setting updating the "default" `health_check_config` preset.

Teleport database agents will cache `health_check_config` resources.

#### Configuration settings

The `health_check_config` resource will expose the following settings:

- Match: Resources that match these matchers will use these health check settings.
    - DBLabels: a label matcher
    - DBLabelsExpression: a predicate label expression
- Interval: time between each health check
- Timeout: the health check connection attempt timeout (timing out fails the check)
- Healthy Threshold: number of consecutive passing health checks after which the resource health status is changed to `healthy`
- Unhealthy Threshold: number of consecutive failing health checks after which the resource health status is changed to `unhealthy`

These settings are sufficient to support TCP connection health checks in the initial implementation.
They are also necessary for other types of health check, including TCP, TLS, and HTTP(S).
Thus, they generalize well.
Future work can extend `health_check_config` with more specific settings as needed to support new health check types.

#### Configuration label matching 

The settings in a `health_check_config` resource will be used for a database if the matchers in `health_check_config.spec.match` match the database.

Both `match.db_labels` and `match.db_labels_expression` may be configured, but at least of them must be configured.
If both are configured, then a database will match only if both matchers match the database, i.e. a logical AND of the matchers.
If only one matcher is configured, then only the configured matcher will be used to match databases.

If multiple `health_check_config` resources match a database, then the `health_check_config` resources will be sorted by name and only the first will apply.

#### Configuration restrictions and defaults

The default for an empty matcher is that it matches nothing.
Users will be required to configure at least one of of the matchers under `spec.match`.
If we make a completely empty `spec.match` default to "match all resources", then it would be easy to inadvertently override settings from other `health_check_config` resources.
If we make a completely empty `spec.match` default to "match nothing", then it would be frustrating for users to discover (after troubleshooting) that by default their custom health check config doesn't do anything.

In future work we may expand health checks to other kinds of resources like desktops or kube clusters.
If a matcher is specified for one type of resource, but not another, then only the configured matcher will match resources.
For example, if `kubernetes_labels` is given, but `db_labels` is not, then the matcher will only match kube clusters and not databases:

    match:
      kubernetes_labels:
      - name: '*'
        values: ['*']

We will provide a default preset `health_check_config` and that preset will match all databases.
The preset default will make it so that most users don't have to configure anything.
If users want to opt-out or change the default settings, then it's easy to do so with `tctl edit health_check_config/default`.

Health check settings determine connection rates to targets and the minimum time between resource health status changes.

We should enforce reasonable health check setting restrictions: 
1. A minimum interval (30s), to prevent an unreasonable or pointless rate of connection attempts
2. A minimum timeout (1s), to give each health check a chance to succeed
3. The healthy/unhealthy thresholds must be greater than 0
4. The maximum interval will be 300s
5. The timeout must be less than or equal to the interval

The defaults we choose should strike a balance between the time to notice a change in connectivity, the rate of connections, and handling unreliable variance of network connectivity.

I reviewed GCP, AWS, and Azure load balancer health check default settings for some comparison.

I also looked at Kubernetes' TCP readiness probes and Docker's healthcheck setting.
The kube/docker settings aren't as comparable to what we are doing, since they are probing containers over a local network, but I think they are still good examples for configuration and status reporting design.

For comparison, here is a table with the default settings for each:

| Name                           | Interval | Timeout              |    Healthy Threshold |  Unhealthy Threshold |
| ------------------------------ | -------- | -------------------- | -------------------- | -------------------- |
| AWS NLB Target Groups [^2]     | 30s      | 10s                  |                    5 |                    2 |
| Azure [^3][^4]                 | 5s       | 5s                   | (not configurable) 1 | (not configurable) 1 |
| GCP [^5]                       | 5s       | 5s                   |                    2 |                    2 |
| Kube Readiness Probe [^6]      | 10s      | 1s                   |                    1 |                    3 |
| Docker [^7]                    | 30s      | 30s                  | (not configurable) 1 |                    3 |
| Teleport                       | 30s      | 5s                   |                    2 |                    1 |

As a brief summary of the Teleport health checker behavior:

1. it does the first check immediately
2. the first health check will always transition to `healthy` or `unhealthy` regardless of configured thresholds
3. it repeats the check after interval time
4. each check blocks until pass or fail

For Teleport the minimum time unknown->healthy or unknown->unhealthy is roughly 0s.

The maximum time is bounded by the timeout: 5s.

Adding the 5s heartbeat polling period to the max times, we get a max time to broadcast the status change: 10s.

Thus, broadcasting the health status for a new or updated resource will take between 0s and 10s.

This is fast enough for web UI enrollment interactions that might wait for the resource network status to be reported.

The default healthy threshold is set to a higher value than the unhealthy threshold to help avoid false positive `healthy` status when network connectivity is unreliable.
It makes sense to bias the health status to `unhealthy` when the network is unreliable anyway.

[^2]: https://docs.aws.amazon.com/elasticloadbalancing/latest/network/target-group-health-checks.html
[^3]: https://learn.microsoft.com/en-us/azure/load-balancer/load-balancer-custom-probe-overview
[^4]: Azure timeouts always match the interval, they cannot be configured independently.
[^5]: https://docs.docker.com/reference/dockerfile/#healthcheck
[^6]: https://cloud.google.com/load-balancing/docs/health-check-concepts
[^7]: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#configure-probes

#### Config example

This is the default preset we will create:

    $ tctl get health_check_config/default
    kind: health_check_config
    version: v1
    metadata:
      name: "default"
      labels:
        teleport.internal/resource-type: preset
    spec:
      match:
        db_labels:
          - name: '*'
            values:
            - '*'

Note that the preset does not specify interval, timeout, nor healthy/unhealthy thresholds.
All of those settings have default values.
Thus, unless those settings are edited by a user, the preset will apply the default settings for the current `teleport` version.
By not specifying the values in the preset config, we can update the defaults in the future and it will only affect users who have not chosen specific settings.

#### Terraform config example

    resource "teleport_health_check_config" "example" {
      version = "v1"
      metadata = {
        name = "example"
      }
      spec = {
        match = {
          db_labels = [{
            name = "*"
            values = ["*"]
          }]
          db_labels_expression = "labels.env != `prod`" 
        }
        healthy_threshold   = 1
        unhealthy_threshold = 1
        interval            = "30s"
        timeout             = "5s"
      }
    }

### DB agent behavior

DB agents will cache `health_check_config` resources.

When a DB agent begins proxying a database, the agent will get all `health_check_config` resources and determine which, if any, will apply to the database.

If health checks are enabled for the database, then the agent will asynchronously start a health checker for that database.

When the agent gathers up info for its `db_server` heartbeat, it will get the current health status of the database from its health checker.

The DB agent's heartbeat system currently polls every 5 seconds for local changes to its `db_server` heartbeat.

When any of the `target_health` fields change, the `db_server` heartbeat will be considered different from the old heartbeat, and the new health status will be announced as an updated `db_server`.

When the database is deregistered from the agent, its health checker will be stopped.

When the database is updated, it will be deregistered and then re-registered, effectively restarting its health checker and health status.

DB agents will also watch `health_check_config` resources.

When `health_check_config` resources are created, deleted, or updated, the DB agent will reevaluate the `health_check_config` label matchers for each registered database.

If the health check settings for a database change, then its current health checker, if any, will be updated to use the new settings.

When a health checker's settings are updated it will reset its interval timer, but it will not reset the health status of the target resource.

The first check after a health check config update will use a larger jitter than normal for the first interval, to avoid aligning all of the agents' health checker intervals too closely.

### Target address resolution

The health check target address will be taken from the database uri.
Teleport does not strictly enforce a host:port database uri, so the target host and/or port may need to be resolved.
The target host and port should generally be resolved in the same way it is for user connections.

As a special case, some databases have multiple endpoints.
For example, a MongoDB replicaset connection string may look like this:

    mongodb://host1:port1,host2:port2,host3:port3/?replicaSet=rs0

To complicate matters further, the user may specify a "secondary" read preference, in which case operations will *only* read from secondary members of the replicaset and fail when secondaries are not available, even if the primary is available:

    mongodb://host1:port1,host2:port2,host3:port3/?replicaSet=rs0&readPreference=secondary

When there are multiple endpoints for a target, the health checker should check every endpoint in parallel and combine the results for each health check.
The combined health check result is a failure if any of the endpoint checks fail.

MongoDB also supports a `mongodb+srv://` scheme, which returns the replicaset "host:port" pairs from a DNS SRV record.
In this case, the target endpoints can change dynamically based on the SRV record returned.
Each health check should lookup the SRV record and check each endpoint returned.

### Health checker behavior

A health checker manager will be added that manages registered resource health check targets.

When a health check target is added, it will set its initial health status to `unknown` with an `initialized` transition reason, then health checker starts it will immediately run its first health check.
It will then wait for an interval of time before checking again.

The health check interval will be jittered to avoid correlated health check times.
Using jitter for the interval is especially important in the face of a health check config update that may affect every database in a cluster at the same time.
The examples throughout this RFD use exact times for illustration purposes, but in reality the interval will be jittered.

Each health check will block the health checker until it either passes or fails, so if the health check takes some time to return, then the next check will occur less than interval time from the failure, possibly immediately.
The maximum time is bounded by the health check timeout, which is itself bounded to be less than the interval.
For example, (Interval=30s, Timeout=10s):

1. health checker starts
2. check times out after 10s - startTime=0s, endTime=10s
3. check times out after 10s - startTime=30s, endTime=40s
4. check times out after 10s - startTime=60s, endTime=70s

Health checks will run periodically as long as the database remains registered with the DB agent.

The health checker will maintain a single counter tracking the number of consecutive passing or failing checks.
If the last check passed but the current check fails, then the counter resets to 1 and tracks the number of consecutive failing checks.
Likewise, if the last check failed but the current check passes, then the counter resets to 1.

If the number of consecutive passing or failing checks reaches the healthy or unhealthy threshold, respectively, then the health status is transitioned to the corresponding status: `healthy` or `unhealthy`.

When the status is `unknown`, a single pass or failure with transition the status to `healthy` or `unhealthy`, regardless of configured thresholds.

Example behavior with the default settings (Interval=30s, Timeout=5s, HealthyThreshold=2, UnhealthyThreshold=1):

1. health checker starts - status=`unknown`, startTime=0s, endTime=0s, count=0, lastErr=nil
2. health check passes   - status=`healthy`, startTime=0s, endTime=0s, count=1, lastErr=nil
3. health check fails    - status=`unhealthy`, startTime=30s, endTime=35s, count=1, lastErr="connection timeout"
4. health check passes   - status=`unhealthy`, startTime=60s, endTime=60s, count=1, lastErr=nil
5. health check passes   - status=`healthy`, startTime=90s, endTime=90s, count=2, lastErr=nil

### Proxy behavior

The proxy currently selects HA agents randomly for user connections.

That will be changed to group the agents by health status, shuffle each group, and then combine the groups to prioritize healthy over unhealthy agent connections.

The priority of health statuses will be:
1. `healthy`
2. `unknown`, `""` (also unknown, older agent)
3. `unhealthy`

The proxy routing prioritization strategy is a fail-open system: if all databases are `unhealthy`, users will still be able to attempt a connection through them, even though their attempt will likely fail.

### Security

Only users who can `tctl get db_server` can see health info, so it's already guarded by RBAC.

The new `health_check_config` resource will be guarded by RBAC.

The following preset Teleport roles will be updated to add new default resource rules:

- `editor`: full read/write permissions on `health_check_config`
- `terraform-provider`: full read/write permissions on `health_check_config`

We will enforce a minimum interval between health checks to prevent accidentally or intentionally dialing too often from agent to database.

### Proto Specification

#### `health_check_config` resource

    // HealthCheckConfig is the configuration for network health checks from an
    // agent to its proxied resource.
    message HealthCheckConfig {
      string kind = 1;
      string sub_kind = 2;
      string version = 3;
      teleport.header.v1.Metadata metadata = 4;
    
      HealthCheckConfigSpec spec = 5;
    }
    
    // HealthCheckConfigSpec is the health check spec.
    message HealthCheckConfigSpec {
      // Match is used to select resources that these settings apply to.
      Matcher match = 1;
      // Timeout is the health check connection establishment timeout.
      // An attempt that times out is a failed attempt.
      google.protobuf.Duration timeout = 2;
      // Interval is the time between each health check.
      google.protobuf.Duration interval = 3;
      // HealthyThreshold is the number of consecutive passing health checks after
      // which a target's health status becomes "healthy".
      uint32 healthy_threshold = 4;
      // UnhealthyThreshold is the number of consecutive failing health checks after
      // which a target's health status becomes "unhealthy".
      uint32 unhealthy_threshold = 5;
    }

    // Matcher is a resource matcher for health check config.
    message Matcher {
      // DBLabels selects the databases to match.
      // The result is logically ANDed with DBLabelsExpression.
      // An empty value matches all databases.
      repeated teleport.label.v1.Label db_labels = 1;
      // DBLabelsExpression is a predicate expression to match databases.
      // The result is logically ANDed with DBLabels.
      // An empty value matches all databases.
      string db_labels_expression = 2;
    }

#### Reporting health status in heartbeat

A new message, TargetHealth, will be added to hold health status information.

TargetHealth will be added as a field to the database heartbeat: `db_server.status.target_health`.
    
    // TargetHealth describes the health status of network connectivity between
    // an agent and a resource.
    message TargetHealth {
      // Address is the target <ip:port>.
      string Address = 1 [(gogoproto.jsontag) = "address,omitempty"];
      // Protocol is the health check protocol such as "tcp".
      string Protocol = 2 [(gogoproto.jsontag) = "protocol,omitempty"];
      // Status is the health status, one of "", "unknown", "healthy", "unhealthy".
      string Status = 3 [(gogoproto.jsontag) = "status,omitempty"];
      // TransitionTimestamp is the time that the last status transition occurred.
      google.protobuf.Timestamp TransitionTimestamp = 4 [
        (gogoproto.jsontag) = "transition_timestamp,omitempty",
        (gogoproto.nullable) = true,
        (gogoproto.stdtime) = true
      ];
      // TransitionReason explains why the last transition occurred.
      string TransitionReason = 5 [(gogoproto.jsontag) = "transition_reason,omitempty"];
      // TransitionError shows the health check error observed when the transition
      // happened. Empty when transitioning to "healthy".
      string TransitionError = 6 [(gogoproto.jsontag) = "transition_error,omitempty"];
      // Message is additional information meant for a user.
      string Message = 7 [(gogoproto.jsontag) = "message,omitempty"];
    }

### Backward Compatibility

Older database agents will not perform health checks, so they will always report health status `""` (unknown), which is equivalent to agents that have disabled health checks.

An `""` or `unknown` health status will not prevent connections to that database through that agent, but it will prioritize `healthy` agents first.

### Audit Events

We will add new audit events for Create/Update/Delete of the `health_check_config` resource.

### Observability

When a health check fails we will emit a log at DEBUG level including the error message.
When status transitions to `healthy` we will emit a log at INFO.
When status transitions to `unhealthy` we will emit a log at WARN level.

Prometheus metrics are intended for performance tracking purposes, but we could nonetheless add metrics tracking the health check pass/fail counts or connection dial latency.
Such metrics may be useful to monitor a particular agent and generate alerts, however I do not think this is justified, especially in the initial implementation, because the health check system itself is an observability feature and users can monitor `db_server` health status using Teleport's API instead.

Distributed tracing (OpenTelemetry) is not needed.

### Test Plan

- Dynamic health check configuration
  - [ ] Dynamic `health_check_config` resource create, read, update, delete operations are supported using `tctl`
  - [ ] Updating `health_check_config` resets `db_server.status.target_health.status` for matching databases
  - [ ] Updating `health_check_config` db labels matcher, such that a database no longer matches the labels, resets that database's `db_server.status.target_health`
  - [ ] Deleting `health_check_config` resets a formerly matched database's `db_server.status.target_health`
- [ ] Configure a database agent with a database with an unreachable uri and health checks enabled. The web UI resource page shows an `unhealthy` indicator for that database.
  - [ ] Without restarting the agent, make the database endpoint reachable and observe that the indicator in the web UI resources page disappears after some time.

Pending approval of the proposed web UI changes, I may add other tests here for the UI features.

### Implementation phases

Health checks will cause additional backend write activity and may pose problems at scale, which adds complexity to implementation, therefore we will implement health checks in two phases:

#### Phase one

We will implement health checks as described, but without updating the heartbeat comparison function to check health status.

Health status changes will therefore only be announced alongside heartbeats incidentally - changes in health status will not trigger an early heartbeat announcement

This will avoid backend scale issues completely.

As a side-effect, UX that relies on quick updates to health status will be excluded from this phase.

#### Phase two

We will update the heartbeat comparison function to check health status.

Health status changes will trigger an early heartbeat announcement.

The auth service's inventory control system will be updated to introduce a new message type: InventoryHealthStatus.

The auth service will also enforce a rate limiting mechanism to avoid excessive burst of write activity on the backend.

The rate limiting mechanism will use a variable rate limit on burst and sustained health status updates: details TBD in future updates to this RFD.

We will then implement UX changes that rely on quick health status updates.
