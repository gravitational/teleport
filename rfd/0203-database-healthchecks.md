---
authors: Gavin Frazar (gavin.frazar@goteleport.com)
state: draft
---

# RFD 0203 - Database Health Checks

## Required Approvals

- Engineering: @r0mant && @greedy52
- Product: @roraback

## What

Teleport database agents will periodically perform health checks to a database's endpoint and report the results as a health status in the agent's `db_server` heartbeat.

When a Teleport proxy routes a user connection, the proxy will prioritize a "healthy" agent->db status over "less healthy" agent->db status if multiple agents proxy the same database.

## Why

The primary motivation for this change is to enable smarter proxy->agent routing when multiple db agents heartbeat the same database (i.e an HA setup [^1]).

[^1]: https://goteleport.com/docs/enroll-resources/database-access/guides/ha/#combined-replicas

Currently, in an HA db agent configuration, the proxy will choose one of the db agents at random to handle a user connnection.

If a subset of the db agents can't reach the database endpoint, then user connection attempts will randomly fail.

To fix this, the Teleport proxy service can sort the db agents by target health status and preferrentially dial agents that have healthy network connectivity to the database.

Secondary motivations for this change:

1. improve database resource onboarding UX
2. surface agent->resource connectivity issues early
3. improve troubleshooting
4. lay the groundwork for collecting agent->resource latency measurements that can be used to make routing decisions based on latency

Related issues:
- [Database Access upstream DB health checks](https://github.com/gravitational/teleport/issues/20544)
- [Desktop Access: support latency-based routing to a desktop service](https://github.com/gravitational/teleport/issues/40905)
- [Detect and report latency measurements for Desktop sessions in the UI](https://github.com/gravitational/teleport/issues/35691)

## Details

### Health status

Database target health status will be stored in the db agent's ephemeral `db_server` heartbeat as the `spec.target_health` field.
The `target_health` will include, among other things, a `status` field.

These are the possible values for the `target_health.status` field:
- ""
- "init"
- "healthy"
- "unhealthy"

The empty status "" is unknown/disabled status.
If health checks are disabled or an older agent is proxying the db, then the health status will be "".

The "init" status is the initial health status and means that the health checking has started but the status has yet to be determined as "healthy" or "unhealthy".

The "healthy" status is reported after the number of consecutive passing health checks has reached a healthy threshold.

The "unhealthy" status is reported after the number of consecutive failing health checks has reached an unhealthy threshold.

As a special case, if the status is currently "init" and a health check fails, then status will change to "unhealthy" even if the unhealthy threshold is larger than 1.
This special case is to bound the amount of time spent in the "init" status if health checks flap between pass/fail without reaching either threshold.
It makes sense to bias the health status to "unhealthy" when the network is unreliable anyway.

### Types of health checks

For the initial implementation we will only support TCP health checks and only for databases.

We can extend this feature later to include additional health check protocols such as HTTP and TLS checks.

We can also extend this feature to other one-agent-to-many-resources types of resources: apps, Windows desktops, OpenSSH servers, etc.

#### TCP health check

A TCP health check will attempt to dial the target address.
If dialing the target address times out or receives a TCP reset (RST), then the check has failed.
Otherwise, the check has passed.

### Agent behavior

When an agent begins proxying a database that has health checks enabled, the agent will start a health checker for that database and set its health status to "init".

When the agent gathers up info for its `db_server` heartbeat, it will get the current health status of the database from its health checker.

The db agent's heartbeat system currently polls every 5 seconds for local changes to its DatabaseServerV3 heartbeat.

When any of the `target_health` fields change, the DatabaseServerV3 heartbeat will be considered different from the old heartbeat, and the new health status will be announced as an updated `db_server`.

When the database is deregistered from the agent, its health checker will be stopped.

When the database is updated, it will be deregistered and then re-registered, effectively restarting its health checker and health status.

### Health checker behavior

A health checker manager will be added that manages a mapping from db name to a single health checker:

    // Target is a health check target.
    type Target struct {
    	// Name is the uniquely identifying name of the target.
    	Name string
    	// Addr is the target's address.
    	Addr string
    	// Spec is the health check configuration to use for the target.
    	Spec types.HealthCheckSpec
    }
    
    // Manager manages health checkers.
    type Manager interface {
    	// AddTarget adds a new target health checker and starts the health checker.
    	AddTarget(ctx context.Context, target Target) error
    	// GetTargetHealth returns the health of a given target.
    	GetTargetHealth(name string) (types.TargetHealth, error)
    	// RemoveTarget removes a given target and stops its health checker.
    	RemoveTarget(name string) error
    	// Close stops all health checkers.
    	Close()
    }

When a health checker starts, it will immediately run its first health check and then wait for an interval of time to check again.

Each health check will block the health checker until it either passes or fails, so if the health check takes some time to return, then the next check will occur less than interval time from the failure, possibly immediately.
The maximum time is bounded by the health check timeout.
Thus, if the timeout is greater than the interval, it can effectively be the interval between checks.
For example, (Interval=10s, Timeout=15s):

1. health checker starts
2. check times out after 15s - startTime=0s, endTime=15s
3. check times out after 15s - startTime=15s, endTime=30s
4. check times out after 15s - startTime=30s, endTime=45s


Health checks will run periodically as long as the database is registered with the db agent.

The health checker will maintain a single counter tracking the number of consecutive passing or failing checks.
If the last check passed but the current check fails, then the counter resets to 1 and tracks the number of consecutive failing checks.
Likewise, if the last check failed but the current check passes, then the counter resets to 1.

If the number of consecutive passing or failing checks reaches the healthy or unhealthy threshold, respectively, then the health status is transitioned to the corresponding status: "healthy" or "unhealthy".

Example behavior with the default settings (Interval=10s, Timeout=5s, HealthyThreshold=2, UnhealthyThreshold=1):

1. Health checker starts - status=init, startTime=0s, endTime=0s, count=0, lastErr=nil
2. health check passes   - status=init, startTime=0s, endTime=0s, count=1, lastErr=nil
3. health check fails    - status=unhealthy, startTime=10s, endTime=15s, count=1, lastErr="connection timeout"
4. health check passes   - status=unhealthy, startTime=20s, endTime=20s, count=1, lastErr=nil
5. health check passes   - status=healthy, startTime=30s, endTime=30s, count=2, lastErr=nil

### Proxy behavior

The proxy currently dials HA agents at random for a user connection.

That will be changed to group the agents by health status, shuffle each group, and then combine the groups to prioritize healthy over unhealthy agent connections.

The priority of health statuses will be:
1. "healthy"
2. "init"
3. "" (unknown)
4. "unhealthy"

The justification for the "healthy" and "unhealthy" status relative ordering should be obvious.
It is perhaps less obvious why "init" should be preferred over "".
By definition an "init" status represents 0 failing checks and 0 or more passing health checks, whereas "" represents no health information at all.
Therefore, the proxy should prefer "init" over "" health status.

### Configuration

Health checks will be an opt-in configurable setting with reasonable defaults.

Health checks should be opt-in to avoid issues for existing customers who upgrade.
However, our docs configuration references should have health checks enabled to encourage usage.

Users can change the default health check settings cluster-wide in `cluster_networking_config`.
The cluster-wide settings will be used unless overridden by health check settings on a specific resource.

#### Configuration settings

Health check will expose the following settings:
- Enabled: whether to enable health checks
- Interval: time between each health check
- Timeout: the health check connection attempt timeout (timing out fails the check)
- Healthy Threshold: number of consecutive passing health checks after which the resource health status is changed to "healthy"
- Unhealth Threshold: number of consecutive failing health checks after which the resource health status is changed to "unhealthy"

#### Configuration restrictions and defaults

Health check settings will determine the rate at which connections are established to targets and the minimum amount of time between resource's health status changes.

We should enforce some reasonable health check setting restrictions: 
1. A minimum interval (1s), to prevent an unreasonable or pointless rate of connection attempts
2. A minimum timeout (1s), to give each health check a chance to succeed
3. The healthy/unhealthy thresholds must be greater than 0

The defaults we choose should strike a balance between the time to notice a change in connectivity, the rate of connnections, and handling unreliable variance of network connectivity.

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
| Teleport                       | 10s      | 5s                   |                    2 |                    1 |

Recall the health checker behavior:
1. it does the first check immediately
2. it repeats the check after interval time
3. each check blocks until pass or fail.
4. a single failure during init will always go to unhealthy state

The minimum time to transition out of init is therefore given by:
   
    interval * (threshold-1)

For Teleport, the minimum time init->healthy is 10s and init->unhealthy is roughly 0s.

The maximum time is given by:

    max(interval, timeout) * (healthy_threshold-1) + timeout
  
The maximum time for both is the same: 15s.

Adding the 5s heartbeat polling period to the max times, we get a max time to broadcast the status change: 20s.

So it will take [0s, 20s] to broadcast a health status for a new or updated resource.

This is fast enough for web UI enrollment interactions that might wait for the resource network status to be reported.

The default healthy threshold is set to a higher value than the unhealthy threshold to help avoid false positive "healthy" status when network connectivity is unreliable.

[^2]: https://docs.aws.amazon.com/elasticloadbalancing/latest/network/target-group-health-checks.html
[^3]: https://learn.microsoft.com/en-us/azure/load-balancer/load-balancer-custom-probe-overview
[^4]: Azure timeouts always match the interval, they cannot be configured independently.
[^5]: https://docs.docker.com/reference/dockerfile/#healthcheck
[^6]: https://cloud.google.com/load-balancing/docs/health-check-concepts
[^7]: https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#configure-probes

#### Dynamic config example

    kind: db
    version: v3
    metadata:
      name: "example"
      labels:
        env: "dev"
    spec:
      protocol: "postgres"
      uri: "localhost:5432"
      health_check:
        enabled: true
        interval: 30s
        timeout: 5s
        healthy_threshold: 3
        unhealthy_threshold: 2
  
#### Static config example

    db_service:
      enabled: true
      databases:
      - name: "example"
        protocol: "postgres"
        uri: "localhost:5432"
        static_labels:
          "env": "dev"
        health_check:
          enabled: true
          interval: 30s
          timeout: 5s
          healthy_threshold: 3
          unhealthy_threshold: 2

#### Terraform config example

    resource "teleport_database" "example" {
      version = "v3"
      metadata = {
        name = "example"
        labels = {
          "env" = "dev"
        }
      }
      spec = {
        protocol = "postgres"
        uri      = "localhost:5432"
        health_check = {
          enabled             = true
          healthy_threshold   = 1
          unhealthy_threshold = 1
          interval            = "30s"
          timeout             = "5s"
        }
      }
    }

#### Discovery config example

Discovery service matchers can specify health check settings for each database that the matcher discovers:

    discovery_service:
      enabled: true
      discovery_group: "teleport-dev-2"
      aws:
        - types:
            - "rds"
          regions:
            - "ca-central-1"
          tags:
            "env": "prod"
          health_check:
            enabled: true # enabled with these settings for each db discovered by this matcher
            healthy_threshold: 1
            unhealthy_threshold: 1
            interval: 30s
            timeout: 5s
        - types:
            - "rds"
          regions:
            - "us-west-1"
            - "us-west-2"
          tags:
            "env": "dev"
          health_check:
            enabled: false # disabled for each db discovered by this matcher (can be omitted, since this is the default)

#### Global config example

    kind: cluster_networking_config
    metadata:
      labels:
        teleport.dev/origin: config-file
      name: cluster-networking-config
      revision: 088012d5-4f2e-4490-b637-00121568e704
    spec:
      client_idle_timeout: 0s
      idle_timeout_message: ""
      keep_alive_count_max: 3
      keep_alive_interval: 5m0s
      proxy_listener_mode: 1
      session_control_timeout: 0s
      tunnel_strategy:
        agent_connection_count: 2
        type: proxy_peering
      web_idle_timeout: 0s
      health_check:
        enabled: true
        interval: 30s
        timeout: 5s
        healthy_threshold: 3
        unhealthy_threshold: 2
    version: v2

### UX

The web UI will be updated to leverage database health status information.

Unhealthy databases will be displayed with a warning icon or tooltip icon indicating that the database is unhealthy.

Recall that there may be multiple `db_server` heartbeats for the same database, but the web UI only shows a single resource tile for the database.
If this is the case, then the tooltip will be shown if any of the heartbeats have an unhealthy `target_health.status`.
The tooltip content will look like this:
    
    m/n Teleport database services proxying access to this database cannot reach the database endpoint.
    <link to healthcheck/network troubleshooting doc>

    Last error: "(tcp) failed: Operation timed out"
    Database:
    - URI: <db uri>
    - IP: <resolved IP>
    - Port: <port>
    Affected Teleport database services:
    - Hostname: <hostname>
      UUID: <uuid>
    - Hostname: <hostname>
      UUID: <uuid>

#### Out of scope

The following ideas were considered, but ultimately cut from the initial implementation of this RFD to reduce its scope:

1. Add a new type of notification routing rule that the auth service can use to send a notification when a target becomes unhealthy, much like the `access_request_routing_rules` added in [RFD 87 - Access request notification routing](./0087-access-request-notification-routing.md).
2. Automatically create a "user task" in the AWS integration dashboard when an AWS RDS database's health status is unhealthy.

These are both good improvements for monitoring and may be added to this RFD in future work.

#### User story: AWS RDS database enrollment

A Teleport cluster admin is new to Teleport and would like to enroll an AWS RDS database with Teleport.

- The user logs into Teleport, sees the "Enroll New Resource" button and clicks that.
- They click on the "RDS PostgreSQL" tile - a guided database enrollment wizard.
- After selecting an AWS integration, region, and VPC, the user sees the "Enroll RDS Database" page.
- On the page is a toggle button labeled "Monitor database endpoint health", which is already enabled by default.
- The user leaves the toggle enabled, selects a database to enroll, and clicks "Next". 
- On the next page, the user is asked to configure an IAM role, choose AWS subnets and security groups, and finally deploy a Teleport database service in ECS using those settings.
- The user selects a public subnet and a security group that allows all outbound traffic, then clicks "Deploy".

The page tells the user that it is waiting for the new db agent to start and join the Teleport cluster:

    Teleport is currently deploying a Database Service.
    It will take at least a minute for the Database Service to be created and joined to your cluster.

After the new agent joins, the page shows a success message:

    Successfully created and detected your new Database Service.
    
The page displays a new message below that:

    Teleport is testing network connectivity between the Database Service and your Database.

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
> Network healthchecks can be used to ensure that the deployment's network settings are correct before the user moves on from the deployment step.
> The other failure modes do not depend on the deployment settings, so it makes sense to keep those tests in a separate subsequent step.

#### User story: web UI resource health indicators

Alice has deployed a Teleport database agent and enrolled a PostgreSQL database with health checks enabled and using the default settings.

Some time later, another user reconfigures the database firewall rules to restrict the allowed inbound traffic IP ranges, but they inadvertantly block the Teleport database agent from reaching the database over the network.

Alice logs into the web UI and navigates to the resources page. 

Alice sees a tooltip or warning icon on the database she set up earlier.

Alice clicks on the tooltip/icon to see the message:

    1/1 Teleport database services proxying access to this database cannot reach the database endpoint.
    <link to healthcheck/network troubleshooting doc>

    Last error: "(tcp) failed: Operation timed out"
    Database:
    - URI: <db uri>
    - IP: <resolved IP>
    - Port: <port>
    Affected Teleport database services:
    - Hostname: <hostname>
      UUID: <uuid>

Among other suggestions, the documentation includes:
- check that the database is actually listening on `<port>`
- check that database inbound TCP traffic is allowed from the agent to `<ip : port>`
- check that agent outbound TCP traffic is allowed to `<ip : port>`

After checking the linked documentation and some investigation Alice diagnoses that the timeout is caused by the database's firewall dropping packets from the agent's IP, which she resolves by updating the firewall rules.

After a short (<20s) time the threshold is reached and the agent changes its status to "healthy".

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
  target_health:
    addr: example.com:5432
    protocol: TCP
    transition_timestamp: "2025-02-19T01:53:39.144218Z"
    transition_reason: "healthy threshold reached"
    status: healthy
  version: 18.0.0-dev
version: v3
```

### Security

Only users who can `tctl get db_server` can see health info, so it's already guarded by RBAC.

We will enforce a minimum interval between health checks to prevent accidentally or intentionally dialing too often from agent to database.

Users who can register or update databases with Teleport can already make a Teleport agent dial that target through other means, so adding health check configuration is no worse.

### Privacy

N/A

### Proto Specification

A new message, HealthCheckSpec, will be added as field named `health_check` in DatabaseSpecV3 and ClusterNetworkingConfigSpecV2.

    // HealthCheckSpec is the configuration for network health checks from an agent
    // to a resource.
    message HealthCheckSpec {
      // Enabled determines if health checks are enabled for this resource.
      bool Enabled = 1 [(gogoproto.jsontag) = "enabled"];
      // Timeout is the health check connection establishment timeout.
      // An attempt that times out is a failed attempt.
      int64 Timeout = 2 [
        (gogoproto.jsontag) = "timeout,omitempty",
        (gogoproto.casttype) = "Duration"
      ];
      // Interval is the time between each health check.
      int64 Interval = 3 [
        (gogoproto.jsontag) = "interval,omitempty",
        (gogoproto.casttype) = "Duration"
      ];
      // HealthyThreshold is the number of consecutive passing health checks after
      // which a target's health status becomes "healthy".
      uint32 HealthyThreshold = 4 [(gogoproto.jsontag) = "healthy_threshold,omitempty"];
      // UnhealthyThreshold is the number of consecutive failing health checks after
      // which a target's health status becomes "unhealthy".
      uint32 UnhealthyThreshold = 5 [(gogoproto.jsontag) = "unhealthy_threshold,omitempty"];
    }
    
A new message, TargetHealth, will be added as a field named `target_health` in DatabaseServerSpecV3.
    
    // TargetHealth describes the health status of network connectivity between
    // an agent and a resource.
    message TargetHealth {
      // Addr is the target <ip:port>.
      string Addr = 1 [(gogoproto.jsontag) = "addr,omitempty"];
      // Protocol is the health check protocol such as "tcp".
      string Protocol = 2 [(gogoproto.jsontag) = "protocol,omitempty"];
      // Status is the health status, one of "", "init", "healthy", "unhealthy".
      string Status = 3 [(gogoproto.jsontag) = "status,omitempty"];
      // TransitionTimestamp is the time that the last status transition occurred.
      google.protobuf.Timestamp TransitionTimestamp = 4 [
        (gogoproto.jsontag) = "transition_timestamp,omitempty",
        (gogoproto.stdtime) = true,
        (gogoproto.nullable) = true
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

Older database agents will not perform health checks, so they will always report health status "" (unknown), which is equivalent to agents that have disabled health checks.

An unknown health status will not prevent connections to that database through that agent, but it will prioritize "healthy" agents first.

### Audit Events

No new audit events will be added.

### Observability

When a health check fails we will emit a log at TRACE level including the error message.
When status transitions to "healthy" we will emit a log at INFO.
When status transitions to "unhealthy" we will emit a log at WARN level.

Prometheus metrics are intended for performance tracking purposes, but we could nonetheless add metrics tracking the health check pass/fail counts or connection dial latency.
Such metrics may be useful to monitor a particular agent and generate alerts, however I do not think this is justified, especially in the initial implementation, because the health check system itself is an observability feature and users can monitor `db_server` health status using Teleport's API instead.

Distributed tracing (OpenTelemetry) is not needed.

### Product Usage

N/A

### Test Plan

- [ ] Configure a database agent with a static database with an unreachable uri and health checks enabled. The web UI resource page shows an "unhealthy" indicator/tooltip for that database.
  - [ ] Without restarting the agent, make the database endpoint reachable and observe that the indicator/tooltip in the web UI resources page disappears after some time.

Pending approval of the proposed web UI changes, I may add other tests here for the UI features.

