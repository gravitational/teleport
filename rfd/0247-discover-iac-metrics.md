---
authors: Charles Bryan (charles@goteleport.com)
state: draft
---

# RFD 0247 - IaC Discovery Flow Product Metrics

## Required Approvers

- Engineering: @r0mant @avatus

## Purpose

Complete instrumentation of the IaC discovery flow by adding new usage events to measure adoption growth, funnel success rates, and enable future product analysis.

## Goals

1. **Track adoption and growth**
   - Current adoption: How many clusters are enrolling resources using discovery?
   - New adopters: How many clusters setup discovery each month?
   - Growth trajectory: What is the month-over-month adoption rate?

2. **Measure setup success and identify blockers**
   - Success rate: What % complete the full funnel?:

     [start] → [copy terraform] → configure discovery → [check integration] → resources discovered → [no issues]

   - Drop-off analysis: Where in the funnel do users fail?
   - Common issues: What issues block users from successful setup?

3. **Ensure enough event coverage for follow-up analysis**
   - Instrument all key discovery events throughout the IAC funnel and integration management
   - Establish baseline metrics for ongoing discovery feature development

## Background

There are three ways resources get enrolled in Teleport through discovery:

1. **IAC flow** -- Users start in the web UI at Integrations, configure resource discovery options, and receive generated Terraform HCL. They run `terraform apply` which creates an Integration and DiscoveryConfig via the Teleport Terraform provider. The official Terraform module applies the `teleport.dev/iac-tool` label to the DiscoveryConfig resource. The discovery service then polls cloud providers and enrolls matching resources.

2. **Guided Discover flow** -- Users start in the web UI on the Enroll new Resource page, select a resource type, and walk through a multi-step setup. The UI creates a DiscoveryConfig directly via API. Available for EC2 auto-enrollment, RDS auto-enrollment, and EKS auto-enrollment.

3. **Static configuration** -- Matchers defined in `teleport.yaml`. No DiscoveryConfig resource is created. The discovery service reads matchers directly from its config file.

Resources created outside of discovery are considered manually enrolled.

## Events Needed

### Event-to-Goal Mapping

| Goal                                   | Events Used                                                                                                                               |
| -------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| **Goal 1: Adoption and Growth**        | ResourceCreateEvent, DiscoveryConfigEvent                                                                                                 |
| **Goal 2: Setup Success and Blockers** | UIIntegrationEnrollStartEvent, UIIntegrationEnrollCodeCopyEvent, DiscoveryConfigEvent, UIIntegrationEnrollVerifyEvent, UserTaskStateEvent |
| **Goal 3: Complete Instrumentation**   | All of the above                                                                                                                          |

### UserTaskStateEvent — (Already exists, no changes needed)

UserTaskStateEvents will be used to track common issues users run into when setting up Discovery, and will be used at the end of the IAC setup funnel to evaluate success.

A `UserTaskStateEvent` is emitted by the UserTask gRPC service (`lib/auth/usertasks/usertasksv1/service.go`) whenever a user task is created, updated, or upserted. User tasks are created by the discovery service when resource enrollment fails. Each event captures `task_type` (`discover-ec2`, `discover-eks`, `discover-rds`), `issue_type`, `state` (`OPEN` or `RESOLVED`), and `instances_count`.

### IntegrationEnroll Events — (Partially exist, requires instrumentation in IAC flow)

The `UIIntegrationEnroll*` event types exist and are used by other integration flows (AWS OIDC, AWS Identity Center, GitHub). The IAC flow does not currently emit any of these events.

The IAC enrollment flow has the following user actions:

1. **Land on page** — User navigates to the AWS Cloud enrollment page.
2. **Configure** — User sets integration name, selects resource types (EC2), configures tag filters, and chooses regions.
3. **Copy Terraform** — User clicks the copy button for the generated Terraform module.
4. **Apply Terraform** — User runs `terraform apply` outside the UI, which creates the Integration and DiscoveryConfig.
5. **Check Integration** — User clicks "Check Integration" to verify the integration was created.

   \*\* Each of these steps may or may not happen, as Users can potentially skip them and use Terraform directly without going through the UI flow.

To fully instrument this flow, the following events are needed:

| Action                    | Event type                                | Status                                    |
| ------------------------- | ----------------------------------------- | ----------------------------------------- |
| Land on page              | `tp.ui.integrationEnroll.start`           | Exists, needs new `AWS_CLOUD` kind        |
| Integration Configuration | - (Assumed when next steps are accounted) |                                           |
| Copy Terraform            | `tp.ui.integrationEnroll.codeCopy`        | Exists, needs new `AWS_CLOUD` kind        |
| Apply Terraform           | - (DiscoveryConfigEvent)                  | Described in DiscoveryConfigEvent section |
| Check Integration         | `tp.ui.integrationEnroll.verify`          | New event required                        |

#### Changes required

Add a new enum value for the AWS Cloud IAC flow:

```protobuf
enum IntegrationEnrollKind {
  // ... existing values ...
  INTEGRATION_ENROLL_KIND_AWS_CLOUD = 30;
}
```

Add corresponding frontend value in `IntegrationEnrollKind`:

```typescript
AwsCloud = 'INTEGRATION_ENROLL_KIND_AWS_CLOUD',
```

Instrument `Integrations/Enroll/Cloud/Aws/EnrollAws.tsx` to emit `UIIntegrationEnroll*` events with `kind = INTEGRATION_ENROLL_KIND_AWS_CLOUD`:

| Action                        | Event type                         | Fields                                                              |
| ----------------------------- | ---------------------------------- | ------------------------------------------------------------------- |
| User lands on enroll page     | `tp.ui.integrationEnroll.start`    | `kind = AWS_CLOUD`                                                  |
| User copies Terraform HCL     | `tp.ui.integrationEnroll.codeCopy` | `kind = AWS_CLOUD`, `type = INTEGRATION_ENROLL_CODE_TYPE_TERRAFORM` |
| User clicks Check Integration | `tp.ui.integrationEnroll.verify`   | `kind = AWS_CLOUD`                                                  |

`tp.ui.integrationEnroll.verify` is a new event type. Add a `UIIntegrationEnrollVerifyEvent` proto with the same `metadata` and `kind` fields as the existing `UIIntegrationEnrollStartEvent`.

### ResourceCreateEvent — (Already exists, requires changes)

ResourceCreateEvents will be used at the end of the IAC funnel to signal a working setup.

`ResourceCreateEvent` is emitted by the discovery service when a resource is enrolled through discovery. Each event captures `resource_type`, `resource_origin`, `cloud_provider`, and database metadata.

One new field is needed: `discovery_config_name` to determine which resources were discovered and created for a specific integration setup. This will allow us to correlate the end of the funnel correctly in case clusters already have other discovery flows running.

#### Changes required

Add `discovery_config_name` anonymized field to the existing `ResourceCreateEvent` proto for joining in the funnel:

```protobuf
// ResourceCreateEvent is emitted when a resource is created.
message ResourceCreateEvent {
  // EXISTING
  // resource_type is the type of resource ("node", "node.openssh", "db", "k8s", "app").
  string resource_type = 1;
  // resource_origin is the origin of the resource ("cloud", "kubernetes").
  string resource_origin = 2;
  // cloud_provider is the cloud provider the resource came from ("AWS", "Azure", "GCP")
  // if resource_origin == "cloud".
  string cloud_provider = 3;
  // database contains additional database information if resource_type == "db".
  DiscoveredDatabaseMetadata database = 4;

  // NEW
  // discovery_config_name is the anonymized name of the DiscoveryConfig that triggered discovery.
  // Empty for teleport.yaml matcher configuration.
  string discovery_config_name = 6;
}
```

**`discovery_config_name`:** anonymized name of the DiscoveryConfig that triggered discovery. Empty for static configuration. Enables joining with `DiscoveryConfigEvent` to ensure attribution to the correct funnel when resources are discovered.

**Athena event type:** `'tp.resource.create'`

### DiscoveryConfigEvent — (New Event)

Emitted when a DiscoveryConfig resource is created, updated, or deleted. Used as a step in the IAC setup funnel and to track integration lifecycle.

```protobuf
enum DiscoveryConfigAction {
  DISCOVERY_CONFIG_ACTION_UNSPECIFIED = 0;
  DISCOVERY_CONFIG_ACTION_CREATE = 1;
  DISCOVERY_CONFIG_ACTION_UPDATE = 2;
  DISCOVERY_CONFIG_ACTION_DELETE = 3;
}

message DiscoveryConfigEvent {
  // action is the operation performed on the DiscoveryConfig.
  DiscoveryConfigAction action = 1;
  // discovery_config_name is the anonymized name of the DiscoveryConfig.
  string discovery_config_name = 2;
  // resource_types is the list of resource types configured for discovery (e.g., "ec2", "rds", "eks").
  repeated string resource_types = 3;
}
```

**`discovery_config_name`:** anonymized name of the DiscoveryConfig, for correlation with `ResourceCreateEvent.discovery_config_name` and the other funnel events.

**`resource_types`:** extracted from the DiscoveryConfig's matchers. Supports multiple types for future multi-resource setups.

Emitted in `CreateDiscoveryConfig()`, `UpdateDiscoveryConfig()`, and `DeleteDiscoveryConfig()` in the discoveryconfig service.

**Athena event type:** `'tp.discovery.config'`

## Initial Draft Queries (unverified)

**Table:** `prehog_events_v1`

**Field extraction:** `json_extract_scalar(event_data, '$.properties["tp.field_name"]')`

### Goal 1: Track Adoption and Growth

#### What percentage of active clusters use discovery?

Active clusters are those that created any resource in the last 30 days. Discovery clusters are those that also have `tp.resource.discovered` events.

```sql
WITH active_clusters AS (
  SELECT DISTINCT json_extract_scalar(event_data, '$.properties["tp.cluster_name"]') AS cluster_name
  FROM prehog_events_v1
  WHERE event_type = 'tp.resource.create'
    AND event_date >= current_date - interval '30' day
),
discovery_clusters AS (
  SELECT DISTINCT json_extract_scalar(event_data, '$.properties["tp.cluster_name"]') AS cluster_name
  FROM prehog_events_v1
  WHERE event_type = 'tp.resource.discovered'
    AND event_date >= current_date - interval '30' day
)
SELECT
  COUNT(DISTINCT d.cluster_name) AS discovery_clusters,
  COUNT(DISTINCT a.cluster_name) AS total_active_clusters,
  CAST(COUNT(DISTINCT d.cluster_name) AS DOUBLE) * 100.0 /
    CAST(COUNT(DISTINCT a.cluster_name) AS DOUBLE) AS adoption_pct
FROM active_clusters a
LEFT JOIN discovery_clusters d ON a.cluster_name = d.cluster_name;
```

#### What is the week-over-week discovery adoption rate?

```sql
WITH weekly_discovery AS (
  SELECT
    date_trunc('week', from_iso8601_timestamp(json_extract_scalar(event_data, '$.timestamp'))) AS week,
    COUNT(DISTINCT json_extract_scalar(event_data, '$.properties["tp.cluster_name"]')) AS clusters
  FROM prehog_events_v1
  WHERE event_type = 'tp.resource.discovered'
    AND event_date >= current_date - interval '90' day
  GROUP BY 1
),
weekly_total AS (
  SELECT
    date_trunc('week', from_iso8601_timestamp(json_extract_scalar(event_data, '$.timestamp'))) AS week,
    COUNT(DISTINCT json_extract_scalar(event_data, '$.properties["tp.cluster_name"]')) AS clusters
  FROM prehog_events_v1
  WHERE event_type = 'tp.resource.create'
    AND event_date >= current_date - interval '90' day
  GROUP BY 1
)
SELECT
  t.week,
  COALESCE(d.clusters, 0) AS discovery_clusters,
  t.clusters AS total_active_clusters,
  CAST(COALESCE(d.clusters, 0) AS DOUBLE) * 100.0 / CAST(t.clusters AS DOUBLE) AS adoption_pct,
  COALESCE(d.clusters, 0) - LAG(COALESCE(d.clusters, 0)) OVER (ORDER BY t.week) AS wow_change
FROM weekly_total t
LEFT JOIN weekly_discovery d ON t.week = d.week
ORDER BY t.week DESC
LIMIT 12;
```

#### How many new clusters adopt discovery each week?

First-time discovery event per cluster.

```sql
WITH first_discovery AS (
  SELECT
    json_extract_scalar(event_data, '$.properties["tp.cluster_name"]') AS cluster_name,
    date_trunc('week', MIN(from_iso8601_timestamp(json_extract_scalar(event_data, '$.timestamp')))) AS first_week
  FROM prehog_events_v1
  WHERE event_type = 'tp.resource.discovered'
  GROUP BY 1
)
SELECT first_week AS week, COUNT(*) AS new_clusters
FROM first_discovery
GROUP BY 1
ORDER BY 1 DESC
LIMIT 12;
```

### Goal 2: Measure Setup Success and Identify Blockers

#### IAC funnel

The IAC discovery funnel has five stages:

1. **Start IAC integration configuration** -- user lands on the IAC enrollment page
2. **Configure** -- user enters integration name, selects resources, regions, and tag filters
3. **Copy Terraform** -- user copies the generated Terraform HCL
4. **Create Integration** -- user runs `terraform apply`, which creates the Integration and DiscoveryConfig
5. **First resource discovered** -- the discovery service finds a matching resource

Stages 1-3 happen in the web UI at `Integrations/Enroll/Cloud/Aws/EnrollAws.tsx`. Stages 4-5 are backend events.

| Stage                        | Event                                                      |
| ---------------------------- | ---------------------------------------------------------- |
| 1. Start IAC configuration   | `tp.ui.integrationEnroll.start` with `kind = AWS_CLOUD`    |
| 2. Configure                 | Implied by reaching stage 3                                |
| 3. Copy Terraform            | `tp.ui.integrationEnroll.codeCopy` with `kind = AWS_CLOUD` |
| 4. Create Integration        | `tp.discovery.config` with `action = 'CREATE'`             |
| 5. First resource discovered | `tp.resource.discovered` with `discovery_method = 'iac'`   |

#### What percentage of clusters that start the IAC flow go on to discover resources?

```sql
WITH stage1_start AS (
  SELECT DISTINCT json_extract_scalar(event_data, '$.properties["tp.cluster_name"]') AS cluster_name
  FROM prehog_events_v1
  WHERE event_type = 'tp.ui.integrationEnroll.start'
    AND json_extract_scalar(event_data, '$.properties["tp.kind"]') = 'AWS_CLOUD'
    AND event_date >= current_date - interval '30' day
),
stage3_copy AS (
  SELECT DISTINCT json_extract_scalar(event_data, '$.properties["tp.cluster_name"]') AS cluster_name
  FROM prehog_events_v1
  WHERE event_type = 'tp.ui.integrationEnroll.codeCopy'
    AND json_extract_scalar(event_data, '$.properties["tp.kind"]') = 'AWS_CLOUD'
    AND event_date >= current_date - interval '30' day
),
stage4_config AS (
  SELECT DISTINCT json_extract_scalar(event_data, '$.properties["tp.cluster_name"]') AS cluster_name
  FROM prehog_events_v1
  WHERE event_type = 'tp.discovery.config'
    AND json_extract_scalar(event_data, '$.properties["tp.action"]') = 'CREATE'
    AND event_date >= current_date - interval '30' day
),
stage5_discovered AS (
  SELECT DISTINCT json_extract_scalar(event_data, '$.properties["tp.cluster_name"]') AS cluster_name
  FROM prehog_events_v1
  WHERE event_type = 'tp.resource.discovered'
    AND json_extract_scalar(event_data, '$.properties["tp.discovery_method"]') = 'iac'
    AND event_date >= current_date - interval '30' day
)
SELECT
  (SELECT COUNT(*) FROM stage1_start) AS started,
  (SELECT COUNT(*) FROM stage3_copy) AS copied_terraform,
  (SELECT COUNT(*) FROM stage4_config) AS created_config,
  (SELECT COUNT(*) FROM stage5_discovered) AS discovered_resources,
  CAST((SELECT COUNT(*) FROM stage3_copy) AS DOUBLE) * 100.0 /
    NULLIF(CAST((SELECT COUNT(*) FROM stage1_start) AS DOUBLE), 0) AS start_to_copy_pct,
  CAST((SELECT COUNT(*) FROM stage4_config) AS DOUBLE) * 100.0 /
    NULLIF(CAST((SELECT COUNT(*) FROM stage3_copy) AS DOUBLE), 0) AS copy_to_config_pct,
  CAST((SELECT COUNT(*) FROM stage5_discovered) AS DOUBLE) * 100.0 /
    NULLIF(CAST((SELECT COUNT(*) FROM stage4_config) AS DOUBLE), 0) AS config_to_discovered_pct;
```

#### What are the most common discovery issues blocking users?

```sql
SELECT
  json_extract_scalar(event_data, '$.properties["tp.task_type"]') AS task_type,
  json_extract_scalar(event_data, '$.properties["tp.issue_type"]') AS issue_type,
  SUM(CAST(json_extract_scalar(event_data, '$.properties["tp.instances_count"]') AS INTEGER)) AS affected_resources,
  COUNT(DISTINCT json_extract_scalar(event_data, '$.properties["tp.cluster_name"]')) AS affected_clusters
FROM prehog_events_v1
WHERE event_type = 'tp.usertask.state'
  AND json_extract_scalar(event_data, '$.properties["tp.state"]') = 'OPEN'
  AND json_extract_scalar(event_data, '$.properties["tp.task_type"]') LIKE 'discover-%'
  AND event_date >= current_date - interval '30' day
GROUP BY 1, 2
ORDER BY affected_resources DESC;
```

### Goal 3: Complete Instrumentation

#### What resource types and cloud providers are being discovered?

```sql
SELECT
  json_extract_scalar(event_data, '$.properties["tp.resource_type"]') AS resource_type,
  json_extract_scalar(event_data, '$.properties["tp.cloud_provider"]') AS cloud_provider,
  COUNT(*) AS resources_discovered,
  COUNT(DISTINCT json_extract_scalar(event_data, '$.properties["tp.cluster_name"]')) AS clusters
FROM prehog_events_v1
WHERE event_type = 'tp.resource.discovered'
  AND event_date >= current_date - interval '30' day
GROUP BY 1, 2, 3
ORDER BY resources_discovered DESC;
```

#### What is the median time from config creation to first resource discovery?

```sql
WITH config_created AS (
  SELECT
    json_extract_scalar(event_data, '$.properties["tp.cluster_name"]') AS cluster_name,
    json_extract_scalar(event_data, '$.properties["tp.discovery_config_name"]') AS config_name,
    MIN(from_iso8601_timestamp(json_extract_scalar(event_data, '$.timestamp'))) AS config_time
  FROM prehog_events_v1
  WHERE event_type = 'tp.discovery.config'
    AND json_extract_scalar(event_data, '$.properties["tp.action"]') = 'CREATE'
    AND event_date >= current_date - interval '30' day
  GROUP BY 1, 2
),
first_discovered AS (
  SELECT
    json_extract_scalar(event_data, '$.properties["tp.cluster_name"]') AS cluster_name,
    json_extract_scalar(event_data, '$.properties["tp.discovery_config_name"]') AS config_name,
    MIN(from_iso8601_timestamp(json_extract_scalar(event_data, '$.timestamp'))) AS discovered_time
  FROM prehog_events_v1
  WHERE event_type = 'tp.resource.discovered'
    AND event_date >= current_date - interval '30' day
  GROUP BY 1, 2
)
SELECT
  approx_percentile(date_diff('minute', c.config_time, d.discovered_time), 0.5) AS median_minutes,
  approx_percentile(date_diff('minute', c.config_time, d.discovered_time), 0.9) AS p90_minutes,
  COUNT(*) AS configs
FROM config_created c
JOIN first_discovered d ON c.cluster_name = d.cluster_name AND c.config_name = d.config_name
WHERE d.discovered_time > c.config_time;
```
