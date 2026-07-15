---
authors: Charles Bryan (charles@goteleport.com)
state: draft
---

# RFD 0247 - IaC Discovery Flow Product Metrics

## Required Approvers

- Engineering: @r0mant @avatus

## Purpose

The goal of this project is to create a Grafana dashboard to answer product questions about and track the success of new Discovery features. To accomplish this goal, instrumentation of usage events in the Infrastructure-as-Code (IaC) discovery flows will be completed. Event specs will be updated and events will be emitted throughout the discovery configuration process. Once event data is being collected, the Grafana dashboard will be created to answer product questions about growth, new customer onboarding, and UX quality.

## What we want to track

1.  **Auto-discovered TPR Growth**

    We want to see the number of Teleport Protected Resources enrolled through auto-discovery growing over time, and understand how that growth breaks down across discovery flows (IaC, guided, and static config). We also want to track how many clusters are actively using discovery to enroll resources, to measure adoption breadth alongside volume.

    Updates to `ResourceCreateEvent` and the new `DiscoveryConfigEvent` will allow us to attribute discovered resources to a specific discovery flow and track volume and active cluster counts over time.

2.  **Onboarding with Discovery**

    We want to deliver an onboarding experience that makes it easy for new customers to enroll all of their resources. To measure this, we need to understand how long it takes new customers to set up discovery, and track the proportion of new customers that set up discovery versus those that don't.

    A new `DiscoveryConfigEvent` will track when discovery is configured. Combined with the existing cluster activation event and `ResourceCreateEvent`, this gives us time-to-setup and adoption rate for new customers.

3.  **Performance of the Discovery Setup UX**

    We want to understand how the IaC setup flow performs end-to-end: what percentage of users complete the full funnel, where they drop off, and what issues block successful setup. The funnel steps are:

    [start] → [copy terraform] → configure discovery → [check integration] → resources discovered → [no issues]

    Steps in `[]` are optional — users can use the Terraform module without going through the UI. Existing `UIIntegrationEnroll*` events will be updated for the IaC flow, and together with `DiscoveryConfigEvent`, `ResourceCreateEvent`, and `UserTaskStateEvent`, the full funnel can be tracked.

## Background

There are three ways resources get enrolled in Teleport through discovery:

1. **IaC Cloud Integration flow** -- Users start in the web UI by choosing the "{Cloud Provider} with Terraform" card when enrolling a new integration. They fill in the form to configure resource discovery options, and receive generated Terraform HCL. Users run `terraform apply` which creates an Integration and DiscoveryConfig via the Teleport Terraform provider. The discovery Terraform module applies the `teleport.dev/iac-tool` label to the DiscoveryConfig resource. The discovery service then polls cloud providers and enrolls matching resources.

2. **Guided Discover flow** -- Users start in the web UI on the Enroll new Resource page, select a resource type, and walk through a multi-step setup. The UI creates a DiscoveryConfig directly via API. Available for EC2 auto-enrollment, RDS auto-enrollment, and EKS auto-enrollment.

3. **Static configuration** -- Matchers defined in `teleport.yaml`. No DiscoveryConfig resource is created. The discovery service reads matchers directly from its config file.

Resources created outside of discovery are considered manually enrolled.

## Events Implementation

### UserTaskStateEvent — (Already exists, needs discovery_config_name for correlation)

UserTaskStateEvents will be used to track common issues users run into when setting up Discovery, and will be used at the end of the IAC setup funnel to evaluate success.

A `UserTaskStateEvent` is emitted by the UserTask gRPC service (`lib/auth/usertasks/usertasksv1/service.go`) whenever a user task is created, updated, or upserted. User tasks are created by the discovery service when resource enrollment fails. Each event captures `task_type` (`discover-ec2`, `discover-eks`, `discover-rds`), `issue_type`, `state` (`OPEN` or `RESOLVED`), and `instances_count`.

To be able to join `UserTaskStateEvent`s to `DiscoveryConfigEvent`s, `UserTaskStateEvent` needs `discovery_config_name` added to it.

```protobuf
message UserTaskStateEvent {
  // ... existing values ...
  // discovery_config_name is the anonymized name of the DiscoveryConfig used when
  // the issue was created
  string discovery_config_name = 5;
}
```

### IntegrationEnroll Events — (Partially exist, requires instrumentation in IAC flow)

The `UIIntegrationEnroll*` event types exist and are used by other integration flows (AWS OIDC, AWS Identity Center, GitHub). The IAC flow does not currently emit any of these events.

The IAC enrollment flow has the following user actions:

1. **Land on page** — User navigates to the AWS Cloud enrollment page.
2. **Configure** — User sets integration name, selects resource types (EC2), configures tag filters, and chooses regions.
3. **Copy Terraform** — User clicks the copy button for the generated Terraform module.
4. **Apply Terraform** — User runs `terraform apply` outside the UI, which creates the Integration and DiscoveryConfig.
5. **Check Integration** — User clicks "Check Integration" to verify the integration was created.

   \*\* UI steps may or may not happen because users can potentially skip them, using Terraform directly without going through the UI flow.

To fully instrument this flow, the following events are needed:

| Action                    | Event type                                | Status                                     |
| ------------------------- | ----------------------------------------- | ------------------------------------------ |
| Land on page              | `tp.ui.integrationEnroll.start`           | Exists, needs new `AWS_CLOUD` kind         |
| Integration Configuration | - (Assumed when next steps are accounted) |                                            |
| Copy Terraform            | `tp.ui.integrationEnroll.codeCopy`        | Exists, needs new `AWS_CLOUD` kind         |
| Apply Terraform           | - (DiscoveryConfigEvent)                  | Described in DiscoveryConfigEvent section  |
| Check Integration         | `tp.ui.integrationEnroll.step`            | Exists, needs `.._VERIFY_INTEGRATION` step |

#### Changes required

Add a new enum value for the AWS Cloud IAC flow:

```protobuf
enum IntegrationEnrollKind {
  // ... existing values ...
  INTEGRATION_ENROLL_KIND_AWS_CLOUD = 30;
  INTEGRATION_ENROLL_KIND_AZURE_CLOUD = 31; // Coming soon, adding now to reduce changes
  INTEGRATION_ENROLL_KIND_GOOGLE_CLOUD = 32; // Coming soon, adding now to reduce changes
}
```

Add corresponding frontend value in `IntegrationEnrollKind`:

```typescript
AwsCloud = 'INTEGRATION_ENROLL_KIND_AWS_CLOUD';
AzureCloud = 'INTEGRATION_ENROLL_KIND_AZURE_CLOUD';
GoogleCloud = 'INTEGRATION_ENROLL_KIND_GOOGLE_CLOUD';
```

And a new step value for verifying the integration:

```protobuf
enum IntegrationEnrollStep {
  // ... existing values ...
  INTEGRATION_ENROLL_STEP_VERIFY_INTEGRATION = 13;
}
```

Add corresponding frontend value in `userEvent/types.ts`:

```typescript
export enum IntegrationEnrollStep {
  // ... existing values ...
  VerifyIntegration = 'INTEGRATION_ENROLL_STEP_VERIFY_INTEGRATION',
}
```

Instrument `Integrations/Enroll/Cloud/Aws/EnrollAws.tsx` to emit `UIIntegrationEnroll*` events with `kind = INTEGRATION_ENROLL_KIND_AWS_CLOUD`:

| Action                        | Event type                         | Fields                                                                                          |
| ----------------------------- | ---------------------------------- | ----------------------------------------------------------------------------------------------- |
| User lands on enroll page     | `tp.ui.integrationEnroll.start`    | `kind = INTEGRATION_ENROLL_KIND_AWS_CLOUD`                                                      |
| User copies Terraform HCL     | `tp.ui.integrationEnroll.codeCopy` | `kind = INTEGRATION_ENROLL_KIND_AWS_CLOUD`, `type = INTEGRATION_ENROLL_CODE_TYPE_TERRAFORM`     |
| User clicks Check Integration | `tp.ui.integrationEnroll.step`     | `kind = INTEGRATION_ENROLL_KIND_AWS_CLOUD`, `step = INTEGRATION_ENROLL_STEP_VERIFY_INTEGRATION` |

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
  string discovery_config_name = 5;
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
  // cloud_providers is the list of cloud providers the config has matchers for.
  repeated string cloud_providers = 4;
  // creation_method specifies the flow used to create the config ("iac" or "guided")
  string creation_method = 5;
}
```

**`creation_method` values:**

| Value      | Meaning                                                |
| ---------- | ------------------------------------------------------ |
| `"iac"`    | DiscoveryConfig with `teleport.dev/iac-tool` label.    |
| `"guided"` | DiscoveryConfig without `teleport.dev/iac-tool` label. |

**`discovery_config_name`:** anonymized name of the DiscoveryConfig, for correlation with `ResourceCreateEvent.discovery_config_name` and the other funnel events.

**`resource_types`:** extracted from the DiscoveryConfig's matchers. Supports multiple types for future multi-resource setups.

Emitted in `CreateDiscoveryConfig()`, `UpdateDiscoveryConfig()`, and `DeleteDiscoveryConfig()` in the discoveryconfig service.

**Athena event type:** `'tp.discovery.config'`

## Implementation Plan

1. Implement event changes in Prehog ([Guide](https://www.notion.so/goteleport/Adding-New-Prehog-Events-2dcfdd3830be809f85a4c1e11a272d76))
2. Implement backend event changes in Teleport
3. Start Grafana dashboard - Adoption and Onboarding
4. Implement frontend `IntegrationEnroll*` changes.
5. Complete dashboard - UX Funnel

## Draft Athena Queries for Dashboard (Won't be vetted until event data is coming in)

### 1. Auto-discovered TPR Growth

#### 1a. Monthly discovered resources (TPR growth + month-over-month)

```sql
WITH monthly AS (
  SELECT
    date_trunc('month', event_time) AS month,
    count(*) AS resources_discovered
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.resource.create'
    AND json_extract_scalar(event_data, '$.properties["tp.origin"]') = 'cloud'
    AND event_date >= date '2025-01-01'
  GROUP BY 1
)
SELECT
  month,
  resources_discovered,
  LAG(resources_discovered) OVER (ORDER BY month) AS prev_month,
  resources_discovered - LAG(resources_discovered) OVER (ORDER BY month) AS mom_change,
  ROUND(
    CAST(resources_discovered - LAG(resources_discovered) OVER (ORDER BY month) AS double)
    / NULLIF(LAG(resources_discovered) OVER (ORDER BY month), 0) * 100
  , 1) AS mom_pct_change
FROM monthly
ORDER BY month DESC
```

#### 1b. Monthly discovered resources by discovery flow (IaC vs guided vs static)

Joins `tp.resource.create` with `tp.discovery.config` on `discovery_config_name` to determine the creation method. Resources with an empty `discovery_config_name` are attributed to static config (teleport.yaml matchers).

```sql
WITH configs AS (
  SELECT DISTINCT
    json_extract_scalar(event_data, '$.distinct_id') AS cluster_id,
    json_extract_scalar(event_data, '$.properties["tp.discovery.config.name"]') AS config_name,
    json_extract_scalar(event_data, '$.properties["tp.discovery.config.creation_method"]') AS creation_method
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.discovery.config'
    AND json_extract_scalar(event_data, '$.properties["tp.discovery.config.action"]') = 'DISCOVERY_CONFIG_ACTION_CREATE'
    AND event_date >= date '2025-01-01'
)
SELECT
  date_trunc('month', r.event_time) AS month,
  COALESCE(c.creation_method, 'static') AS discovery_flow,
  count(*) AS resources_discovered
FROM "prehog_events_database"."prehog_events_v1" r
LEFT JOIN configs c
  ON json_extract_scalar(r.event_data, '$.distinct_id') = c.cluster_id
  AND json_extract_scalar(r.event_data, '$.properties["tp.discovery.config.name"]') = c.config_name
WHERE r.event_type = 'tp.resource.create'
  AND json_extract_scalar(r.event_data, '$.properties["tp.origin"]') = 'cloud'
  AND r.event_date >= date '2025-01-01'
GROUP BY 1, 2
ORDER BY 1 DESC, 3 DESC
```

#### 1c. Monthly clusters using discovery (adoption + month-over-month)

```sql
WITH monthly AS (
  SELECT
    date_trunc('month', event_time) AS month,
    count(DISTINCT json_extract_scalar(event_data, '$.distinct_id')) AS clusters
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.resource.create'
    AND json_extract_scalar(event_data, '$.properties["tp.origin"]') = 'cloud'
    AND event_date >= date '2025-01-01'
  GROUP BY 1
)
SELECT
  month,
  clusters,
  LAG(clusters) OVER (ORDER BY month) AS prev_month,
  clusters - LAG(clusters) OVER (ORDER BY month) AS mom_change,
  ROUND(
    CAST(clusters - LAG(clusters) OVER (ORDER BY month) AS double)
    / NULLIF(LAG(clusters) OVER (ORDER BY month), 0) * 100
  , 1) AS mom_pct_change
FROM monthly
ORDER BY month DESC
```

### 2. Onboarding with Discovery

#### 2a. Monthly time from cluster activation to first discovered resource

Measures how long it takes new customers to go from cluster activation (`tp.subscription.create`) to their first resource enrolled via discovery. Grouped by activation month so trends can be tracked over time.

```sql
WITH cluster_activated AS (
  SELECT
    json_extract_scalar(event_data, '$.distinct_id') AS cluster_id,
    MIN(event_time) AS activated_at
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.subscription.create'
    AND event_date >= date '2025-01-01'
  GROUP BY 1
),
first_discovered_resource AS (
  SELECT
    json_extract_scalar(event_data, '$.distinct_id') AS cluster_id,
    MIN(event_time) AS first_resource_at
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.resource.create'
    AND json_extract_scalar(event_data, '$.properties["tp.origin"]') = 'cloud'
    AND event_date >= date '2025-01-01'
  GROUP BY 1
),
cluster_discovery_method AS (
  SELECT DISTINCT
    json_extract_scalar(event_data, '$.distinct_id') AS cluster_id,
    FIRST_VALUE(json_extract_scalar(event_data, '$.properties["tp.discovery.config.creation_method"]'))
      OVER (PARTITION BY json_extract_scalar(event_data, '$.distinct_id') ORDER BY event_time) AS first_creation_method
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.discovery.config'
    AND json_extract_scalar(event_data, '$.properties["tp.discovery.config.action"]') = 'DISCOVERY_CONFIG_ACTION_CREATE'
    AND event_date >= date '2025-01-01'
),
time_to_first AS (
  SELECT
    date_trunc('month', ca.activated_at) AS activation_month,
    COALESCE(cdm.first_creation_method, 'static') AS discovery_flow,
    date_diff('hour', ca.activated_at, fr.first_resource_at) AS hours_to_first_resource
  FROM cluster_activated ca
  INNER JOIN first_discovered_resource fr ON ca.cluster_id = fr.cluster_id
  LEFT JOIN cluster_discovery_method cdm ON ca.cluster_id = cdm.cluster_id
)
SELECT
  activation_month,
  discovery_flow,
  count(*) AS clusters,
  approx_percentile(hours_to_first_resource, 0.50) AS median_hours,
  approx_percentile(hours_to_first_resource, 0.75) AS p75_hours,
  approx_percentile(hours_to_first_resource, 0.95) AS p95_hours,
  avg(hours_to_first_resource) AS avg_hours
FROM time_to_first
GROUP BY 1, 2
ORDER BY 1 DESC, 2
```

#### 2b. Monthly clusters that activated but never discovered a resource

```sql
WITH cluster_activated AS (
  SELECT
    json_extract_scalar(event_data, '$.distinct_id') AS cluster_id,
    date_trunc('month', MIN(event_time)) AS activation_month
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.subscription.create'
    AND event_date >= date '2025-01-01'
  GROUP BY 1
),
clusters_with_discovery_config AS (
  SELECT DISTINCT
    json_extract_scalar(event_data, '$.distinct_id') AS cluster_id
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.discovery.config'
    AND json_extract_scalar(event_data, '$.properties["tp.discovery.config.action"]') = 'DISCOVERY_CONFIG_ACTION_CREATE'
    AND event_date >= date '2025-01-01'
),
clusters_with_discovered_resources AS (
  SELECT DISTINCT
    json_extract_scalar(event_data, '$.distinct_id') AS cluster_id
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.resource.create'
    AND json_extract_scalar(event_data, '$.properties["tp.origin"]') = 'cloud'
    AND event_date >= date '2025-01-01'
)
SELECT
  ca.activation_month,
  count(*) AS total_activated_clusters,
  count(cdc.cluster_id) AS configured_discovery,
  count(cdr.cluster_id) AS discovered_resources,
  count(*) - count(cdc.cluster_id) AS never_configured_discovery,
  count(cdc.cluster_id) - count(cdr.cluster_id) AS configured_but_no_resources,
  ROUND(CAST(count(cdr.cluster_id) AS double) / count(*) * 100, 1) AS pct_activated_to_discovered
FROM cluster_activated ca
LEFT JOIN clusters_with_discovery_config cdc ON ca.cluster_id = cdc.cluster_id
LEFT JOIN clusters_with_discovered_resources cdr ON ca.cluster_id = cdr.cluster_id
GROUP BY 1
ORDER BY 1 DESC
```

### 3. Performance of the discovery setup UX

#### 3a. IaC setup funnel — per-cluster progression by month

Based on ALL clusters that entered the IaC funnel at any point (UI start or config creation). Each step is LEFT JOINed so clusters that drop off early are still counted. The `furthest_step` tracks how far each cluster progressed, and drop-off columns show where clusters stopped.

Funnel steps: `[start]` → `[copy terraform]` → `configure discovery` → `[check integration]` → `resources discovered` → `no issues`

```sql
WITH step_start AS (
  SELECT
    json_extract_scalar(event_data, '$.distinct_id') AS cluster_id,
    MIN(event_time) AS step_at
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.ui.integrationEnroll.start'
    AND json_extract_scalar(event_data, '$.properties["tp.integration_enroll.metadata.kind"]') = 'INTEGRATION_ENROLL_KIND_AWS_CLOUD'
    AND event_date >= date '2025-01-01'
  GROUP BY 1
),
step_copy AS (
  SELECT DISTINCT json_extract_scalar(event_data, '$.distinct_id') AS cluster_id
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.ui.integrationEnroll.codeCopy'
    AND json_extract_scalar(event_data, '$.properties["tp.integration_enroll.metadata.kind"]') = 'INTEGRATION_ENROLL_KIND_AWS_CLOUD'
    AND event_date >= date '2025-01-01'
),
iac_configs AS (
  SELECT
    json_extract_scalar(event_data, '$.distinct_id') AS cluster_id,
    json_extract_scalar(event_data, '$.properties["tp.discovery.config.name"]') AS config_name,
    MIN(event_time) AS configured_at
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.discovery.config'
    AND json_extract_scalar(event_data, '$.properties["tp.discovery.config.action"]') = 'DISCOVERY_CONFIG_ACTION_CREATE'
    AND json_extract_scalar(event_data, '$.properties["tp.discovery.config.creation_method"]') = 'iac'
    AND event_date >= date '2025-01-01'
  GROUP BY 1, 2
),
step_verify AS (
  SELECT DISTINCT json_extract_scalar(event_data, '$.distinct_id') AS cluster_id
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.ui.integrationEnroll.step'
    AND json_extract_scalar(event_data, '$.properties["tp.integration_enroll.metadata.kind"]') = 'INTEGRATION_ENROLL_KIND_AWS_CLOUD'
    AND json_extract_scalar(event_data, '$.properties["tp.integration_enroll.step"]') = 'INTEGRATION_ENROLL_STEP_VERIFY_INTEGRATION'
    AND event_date >= date '2025-01-01'
),
step_resources AS (
  SELECT DISTINCT
    json_extract_scalar(event_data, '$.distinct_id') AS cluster_id,
    json_extract_scalar(event_data, '$.properties["tp.discovery.config.name"]') AS config_name
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.resource.create'
    AND json_extract_scalar(event_data, '$.properties["tp.origin"]') = 'cloud'
    AND json_extract_scalar(event_data, '$.properties["tp.discovery.config.name"]') != ''
    AND event_date >= date '2025-01-01'
),
open_issues AS (
  SELECT DISTINCT
    json_extract_scalar(event_data, '$.distinct_id') AS cluster_id,
    json_extract_scalar(event_data, '$.properties["tp.discovery.config.name"]') AS config_name
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.usertask.state'
    AND json_extract_scalar(event_data, '$.properties["tp.usertask.state"]') = 'OPEN'
    AND event_date >= date '2025-01-01'
),
-- Deduplicate to cluster level for steps that need config_name joins
iac_clusters AS (
  SELECT cluster_id, MIN(configured_at) AS configured_at
  FROM iac_configs GROUP BY 1
),
iac_resources AS (
  SELECT DISTINCT sr.cluster_id
  FROM step_resources sr
  INNER JOIN iac_configs ic ON sr.cluster_id = ic.cluster_id AND sr.config_name = ic.config_name
),
iac_open_issues AS (
  SELECT DISTINCT oi.cluster_id
  FROM open_issues oi
  INNER JOIN iac_configs ic ON oi.cluster_id = ic.cluster_id AND oi.config_name = ic.config_name
),
-- Base: all clusters that entered the funnel via UI start OR config creation
funnel_entries AS (
  SELECT cluster_id, MIN(entered_at) AS entered_at FROM (
    SELECT cluster_id, step_at AS entered_at FROM step_start
    UNION ALL
    SELECT cluster_id, configured_at AS entered_at FROM iac_clusters
  ) t
  GROUP BY 1
),
cluster_funnel AS (
  SELECT
    fe.cluster_id,
    fe.entered_at,
    CASE WHEN ss.cluster_id IS NOT NULL THEN 1 ELSE 0 END AS reached_start,
    CASE WHEN sc.cluster_id IS NOT NULL THEN 1 ELSE 0 END AS reached_copy,
    CASE WHEN icc.cluster_id IS NOT NULL THEN 1 ELSE 0 END AS reached_config,
    CASE WHEN sv.cluster_id IS NOT NULL THEN 1 ELSE 0 END AS reached_verify,
    CASE WHEN ir.cluster_id IS NOT NULL THEN 1 ELSE 0 END AS reached_resources,
    CASE WHEN ir.cluster_id IS NOT NULL AND ioi.cluster_id IS NULL THEN 1 ELSE 0 END AS reached_no_issues,
    GREATEST(
      CASE WHEN ss.cluster_id IS NOT NULL THEN 1 ELSE 0 END,
      CASE WHEN sc.cluster_id IS NOT NULL THEN 2 ELSE 0 END,
      CASE WHEN icc.cluster_id IS NOT NULL THEN 3 ELSE 0 END,
      CASE WHEN sv.cluster_id IS NOT NULL THEN 4 ELSE 0 END,
      CASE WHEN ir.cluster_id IS NOT NULL THEN 5 ELSE 0 END,
      CASE WHEN ir.cluster_id IS NOT NULL AND ioi.cluster_id IS NULL THEN 6 ELSE 0 END
    ) AS furthest_step
  FROM funnel_entries fe
  LEFT JOIN step_start ss ON fe.cluster_id = ss.cluster_id
  LEFT JOIN step_copy sc ON fe.cluster_id = sc.cluster_id
  LEFT JOIN iac_clusters icc ON fe.cluster_id = icc.cluster_id
  LEFT JOIN step_verify sv ON fe.cluster_id = sv.cluster_id
  LEFT JOIN iac_resources ir ON fe.cluster_id = ir.cluster_id
  LEFT JOIN iac_open_issues ioi ON fe.cluster_id = ioi.cluster_id
)
SELECT
  date_trunc('month', entered_at) AS month,
  count(*) AS entered_funnel,
  -- Clusters that reached each step
  sum(reached_start) AS reached_start,
  sum(reached_copy) AS reached_copy,
  sum(reached_config) AS reached_config,
  sum(reached_verify) AS reached_verify,
  sum(reached_resources) AS reached_resources,
  sum(reached_no_issues) AS reached_no_issues,
  -- Drop-off: clusters whose furthest step was this one
  count_if(furthest_step = 1) AS dropped_after_start,
  count_if(furthest_step = 2) AS dropped_after_copy,
  count_if(furthest_step = 3) AS dropped_after_config,
  count_if(furthest_step = 4) AS dropped_after_verify,
  count_if(furthest_step = 5) AS stuck_with_issues,
  count_if(furthest_step = 6) AS completed
FROM cluster_funnel
GROUP BY 1
ORDER BY 1 DESC
```

#### 3b. Drop-off analysis — conversion rates by month

Two-segment conversion rates derived from the same funnel. **Before config** measures UI setup drop-off (% of `entered_funnel`). **After config** measures post-setup success (% of `reached_config`).

```sql
-- Uses the same steps as 3a (step_start through cluster_funnel)
-- ...
WITH funnel AS (
  SELECT
    date_trunc('month', entered_at) AS month,
    count(*) AS entered_funnel,
    sum(reached_start) AS reached_start,
    sum(reached_copy) AS reached_copy,
    sum(reached_config) AS reached_config,
    sum(reached_verify) AS reached_verify,
    sum(reached_resources) AS reached_resources,
    sum(reached_no_issues) AS reached_no_issues
  FROM cluster_funnel
  GROUP BY 1
)
SELECT
  month,
  -- Before config: UI setup drop-off (% of entered_funnel)
  entered_funnel,
  ROUND(CAST(reached_start AS double) / NULLIF(entered_funnel, 0) * 100, 1) AS pct_started,
  ROUND(CAST(reached_copy AS double) / NULLIF(entered_funnel, 0) * 100, 1) AS pct_copied,
  ROUND(CAST(reached_config AS double) / NULLIF(entered_funnel, 0) * 100, 1) AS pct_configured,
  -- After config: post-setup success (% of reached_config)
  reached_config,
  ROUND(CAST(reached_verify AS double) / NULLIF(reached_config, 0) * 100, 1) AS pct_verified,
  ROUND(CAST(reached_resources AS double) / NULLIF(reached_config, 0) * 100, 1) AS pct_resources,
  ROUND(CAST(reached_no_issues AS double) / NULLIF(reached_config, 0) * 100, 1) AS pct_no_issues
FROM funnel
ORDER BY month DESC
```

#### 3c. Common issues blocking successful setup by month

Groups `UserTaskStateEvent`s by issue type and task type for IaC discovery configs. Shows the most frequent issues and how many remain unresolved.

```sql
WITH iac_config_names AS (
  SELECT DISTINCT
    json_extract_scalar(event_data, '$.distinct_id') AS cluster_id,
    json_extract_scalar(event_data, '$.properties["tp.discovery.config.name"]') AS config_name
  FROM "prehog_events_database"."prehog_events_v1"
  WHERE event_type = 'tp.discovery.config'
    AND json_extract_scalar(event_data, '$.properties["tp.discovery.config.creation_method"]') = 'iac'
    AND json_extract_scalar(event_data, '$.properties["tp.discovery.config.action"]') = 'DISCOVERY_CONFIG_ACTION_CREATE'
    AND event_date >= date '2025-01-01'
),
issues AS (
  SELECT
    date_trunc('month', ut.event_time) AS month,
    json_extract_scalar(ut.event_data, '$.distinct_id') AS cluster_id,
    json_extract_scalar(ut.event_data, '$.properties["tp.usertask.task_type"]') AS task_type,
    json_extract_scalar(ut.event_data, '$.properties["tp.usertask.issue_type"]') AS issue_type,
    json_extract_scalar(ut.event_data, '$.properties["tp.usertask.state"]') AS state,
    CAST(json_extract_scalar(ut.event_data, '$.properties["tp.usertask.discover_ec2.instances_count"]') AS bigint) AS instances_count
  FROM "prehog_events_database"."prehog_events_v1" ut
  INNER JOIN iac_config_names ic
    ON json_extract_scalar(ut.event_data, '$.distinct_id') = ic.cluster_id
    AND json_extract_scalar(ut.event_data, '$.properties["tp.discovery.config.name"]') = ic.config_name
  WHERE ut.event_type = 'tp.usertask.state'
    AND ut.event_date >= date '2025-01-01'
)
SELECT
  month,
  task_type,
  issue_type,
  count(*) AS total_events,
  count_if(state = 'OPEN') AS opened,
  count_if(state = 'RESOLVED') AS resolved,
  count(DISTINCT cluster_id) AS clusters_affected,
  sum(instances_count) AS total_instances_affected
FROM issues
GROUP BY 1, 2, 3
ORDER BY 1 DESC, total_events DESC
```
