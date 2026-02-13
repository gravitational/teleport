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

1.  **Track adoption and growth**
    - Current adoption: How many clusters are enrolling resources using discovery?
    - New adopters: How many clusters setup discovery each month?
    - Growth trajectory: What is the month-over-month adoption rate?

2.  **Measure setup success and identify blockers**
    - Success rate: What % complete the full funnel?:

      [start] → [copy terraform] → configure discovery → [check integration] → resources discovered → [no issues]

      \*\* steps in `[]` may not happen because users can use the terraform module without the UI.

    - Drop-off analysis: Where in the funnel do users fail?
    - Common issues: What issues block users from successful setup?

3.  **Ensure enough event coverage for follow-up analysis**
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
| **Goal 3: Key Event Coverage**         | All of the above                                                                                                                          |

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

   \*\* UI steps may or may not happen because users can potentially skip them, using Terraform directly without going through the UI flow.

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

**`creation_method` values:**

| Value      | Meaning                                                |
| ---------- | ------------------------------------------------------ |
| `"iac"`    | DiscoveryConfig with `teleport.dev/iac-tool` label.    |
| `"guided"` | DiscoveryConfig without `teleport.dev/iac-tool` label. |

**`discovery_config_name`:** anonymized name of the DiscoveryConfig, for correlation with `ResourceCreateEvent.discovery_config_name` and the other funnel events.

**`resource_types`:** extracted from the DiscoveryConfig's matchers. Supports multiple types for future multi-resource setups.

Emitted in `CreateDiscoveryConfig()`, `UpdateDiscoveryConfig()`, and `DeleteDiscoveryConfig()` in the discoveryconfig service.

**Athena event type:** `'tp.discovery.config'`
