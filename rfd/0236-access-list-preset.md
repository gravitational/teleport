---
author: Lisa Kim (lisa@goteleport.com) and Marek Smoliński (marek@goteleport.com)
state: draft
---

# RFD 0236 - Access Lists Created with Presets

# Required Approvers

- Engineering: @r0mant || @smallinsky (marek) && @fspmarshall (forrest)
- Product: @r0mant

## What

Access lists allows an admin to grant additional roles (and traits) to a group of users on a long lived basis. Currently, to define grants for an access list, admins must select roles that already exists. This requires the admins to know what a role is and know how to create or update a role to customize access.

This RFD proposes creating different guided access lists provided by Teleports web app, where it removes the need for admins to know about roles.

The guide can also be used as a Terraform access list script generator.

## Why

Improves access list usability especially for day one users. Guided access list allows an admin to focus on users and what resources users should have access to. It removes the need for an admin to learn how to create/update roles and removes how roles have any relation to an access list because Teleport will do it for them.

### Presets

Presets will describe the type of guides that admins can use. It describes what kinds of actions that Teleport will take. More info on preset in user stories below.

### User story: As an admin, I want to grant a select group of users (local or Okta imported) long term access to resources

The admin can create an access list using the preset `long-term`.

Preset `long-term` represents an access list that grants members long lived access to Teleport resources. Owners can review access requests. This access list is pretty similar to how access list works now (non-integrated types).

All admin needs to do for this preset is to define the metadata, owners, and members of the access list and define the access to resources (e.g. node_labels and node logins) and Teleport will create the necessary roles and assign them to appropriate member/owner grants in a access list.

### User story: As an admin, I want to require a select group of users (local or Okta imported) to request for short-term access to resources

The admin can create an access list using the preset `short-term`.

Preset `short-term` represents an access list that utilizes JIT. Owners of the access list are reviewers. Members of the access list are requesters that are required to request access to resources and then upon approval are granted short-term access to requested Teleport resources.

All admin needs to do for this preset is to define the metadata, owners, and members of the access list and define the access to resources (e.g. node_labels and node logins) and Teleport will create the necessary roles and assign them to appropriate member/owner grants in a access list.

### Web UI/UX

The Web UI guide has two parts:

#### Defining the `access_list` resource

The guide will ask the admin for input that defines the owners, members, and metadata (e.g. title, description, audit frequency) of an access list. It is the same type of inputs asked of an admin currently in the web UI, but now these guides will break these inputs into smaller multi steps allowing to add clarifications for each step.

#### Defining the allow specs of `role` resources

Only a part of the `role` resource requires admin input and that is the [specs of a role](https://github.com/gravitational/teleport/blob/85829ad33c3af785a220efc0e8a8b46af102f039/api/proto/teleport/legacy/types/types.proto#L3537) specifically the `allow` field which defines giving access to resources.

Instead of expecting the admin to define the `role` spec using traditional YAML or through a long form of input fields, the guide will provide a more [interactive UI/UX](https://www.figma.com/design/CzxaBHF8hirrYv2bTVa4Rw/Identity-Governance?node-id=7973-166743&t=A3qT6ANUJF67SSsr-0) way of defining the role specs. To the admin, the guide will just look like they are "defining access to resources", not defining a role. The fact that roles are being created/edited under the hood is hidden from the admin.

Defining the access to resources (role spec allow field) is split into two steps:

1. Define access that allows users to see the resources.
   - Application (minus some sub_kinds), database, desktop, SSH server, and Kubernetes access is determined by a set of matching `labels` e.g. `env: staging`.
   - AWS IC Application (a sub_kind of an application) access is determined by `app_labels` AND a list of [account assignments](https://github.com/gravitational/teleport/blob/85829ad33c3af785a220efc0e8a8b46af102f039/api/proto/teleport/legacy/types/types.proto#L4059) (AWS account ID + ARN)
   - Git server access is determined by a list of GitHub organization names

1. Sometimes step 1 alone is enough for a user to also connect to the resources they see (e.g. launching basic web applications) but most require this second step as well which is to define the resource principals or identities required to `connect` to resources e.g. SSH server logins, database names and users etc.

#### Dynamic Feedback

The guide will provide dynamic feedback as the admin is defining/tweaking access to resources in the form of:

- Listing the resources given some admin input. E.g. if an admin was defining access for applications, and the admin specifies label `env: staging`, the feedback is to list only applications with matching label.
- If the cluster has `Identity Security` feature enabled, the admin will also be able to use access graph to see a graph of accessible resources.

The dynamic feedback allows admin to essentially preview what members of this access list will see in their own account.

#### Terraform Support

The guide will provide a copyable Terraform script that is equivalent to going through the guide to create an access list with a preset. At the end of the guide, the admin can choose to use the Terraform script only, or let the web app proceed to finish the request.

Similar to dynamic feedback, as the admin is defining/tweaking inputs in the guide, the Terraform script will dynamically update to reflect the latest changes. This script will be visible throughout the entire guide (can optionally be hidden).

The Terraform script will also be available later when viewing the access list created with a preset.

### Security

We expect `admins` to have full permissions but there can be edge cases where their role may limit some access. To prevent users (admins) from overstepping the boundaries set by their assigned roles, the following restrictions are placed when user wants to perform create, delete, or update operations on an access list created with a preset:

#### Backend

- The user is required to have access to perform all CRUD operations on a role otherwise upsert request with preset will fail. The user may not be directly performing actions on a role, but Teleport will on behalf of the user.
- The [same existing rules](https://github.com/gravitational/teleport.e/blob/e49a5ad654408ce0779622c38c7acda0417bfef0/lib/accesslist/service.go#L1648) are applied when creating/deleting/updating an access list. E.g. for updating an access list the user will have to either be a owner of the list or have access to `access_list` resource rules.

#### Web UI

In the web UI, frontend validations will be performed that prevent the user from going forward with the guide if validations fail. When defining or modifying access to resources in the web UI, the user is limited to what their own roles allow them. For example, lets say a few application resources exists in a Teleport cluster:

- If the users currently assigned role does not define access to applications (missing `app_labels`) and therefore unable to list applications, the user will not be able to define any access to application
- If the user defines a label and it produces no resource query results, the user will be prompted to remove the label or try a different label (user will not be able to proceed to the next step in the guide)
  - Similarly if an access list with preset was created, and later a user with lesser permission than the original creator comes in to edit the access to resources, if with those existing access definitions produces no resource query result, the user will be prompted to remove or try other selections (user will not be able to proceed)
- When modifying access to resources for an access list, the same guide will be provided, unless the web app detected unsupported modifications made on a role meant for an access list. If unsupported modifications (or unsupported role versions) were detected, the web UI will point the user to use the standard role editor instead (how normal access list essentially works). This is because the guide does not support parsing all fields of a role.
- If the users currently assigned role defines a `app_label` giving access to just `some` of the applications, the user will be allowed to define access to the resources they can see (this does not prevent them from defining access to resources they can't see, more below)

These frontend validations does not `prevent` a user from granting a resource that they can't `see`. For example, users will be allowed to set `wildcards` that allow all resources. The user's own role may not allow them to see all resources available but use of `wildcards` or even unknowingly selecting a label that is shared between a resource they see and a resource they can't see, all results in granting more access then the user may have intended. To help with surprises like these, the web UI will render a warning that lets users know that the access they define may grant more resources then they intended:

```
Previewing Resources

When defining access for a resource, we will attempt to list a preview of what resources the member may see in their own account. Note that this preview is dependent on what your own roles allow you to see. Access to more resources than what are seen here may be granted.
```

### API

New web API endpoints are created for access list preset operations:

{/_ spell-checker: disable _/}

- `POST /enterprise/accesslistpreset` - Create an access list with a preset
- `PUT /enterprise/accesslistpreset/:accessListId` - Update an access list with a preset

{/_ spell-checker: enable _/}

#### Request Structure

The POST and PUT endpoints accept an `AccessListWithPresetRequest`:

```go
type UIAccessListWithPresetRequest struct {
	// PresetType specifies the preset configuration ("long-term" or "short-term")
	PresetType string `json:"presetType,omitempty"`
	// AccessList contains the access list configuration (metadata, owners, audit settings)
	AccessList *accesslist.AccessList `json:"accessList,omitempty"`
	// Members is the list of members to add to the access list
	Members []*accesslist.AccessListMember `json:"members,omitempty"`
	// AccessRoles defines the role specifications for resource access
	AccessRoles []types.Role `json:"accessRoles,omitempty"`
}
```

#### Response Structure

The endpoints return an `AccessListWithPresetResponse`:

```go
type UIAccessListWithPresetResponse struct {
	// AccessList is the created/updated access list with grants configured
	AccessList *AccessList `json:"accessList,omitempty"`
	// AccessRoles contains the roles defining resource access permissions
	AccessRoles []types.Role `json:"accessRoles,omitempty"`
	// RequesterRole is the role allowing members to request access
	RequesterRole types.Role `json:"requesterRole,omitempty"`
	// ReviewerRole is the role allowing owners to review access requests
	ReviewerRole types.Role `json:"reviewerRole,omitempty"`
	// Members are the access list members
	Members []*accesslist.AccessListMember `json:"members,omitempty"`
}
```

#### Roles Created and Limits

In most use cases, the proxy will create three types of roles: one requester role, one reviewer role, and one access role. Multiple access roles are supported for flexibility when defining access to resources with incompatible label requirements (see "Defining multiple role specs" section below for details). However, for typical scenarios, the number of access roles should not exceed 10 to maintain manageability and avoid excessive role proliferation. The flow will enforce limits on the total number of roles created per access list preset operation.

#### Preset Type

The requested preset will determine what kind of actions Teleport will perform on behalf of the user.

```proto
// PresetType describes what type of preset was requested.
enum PresetType {
  // PRESET_TYPE_UNSPECIFIED is the zero value.
  PRESET_TYPE_UNSPECIFIED = 0;
  // PRESET_TYPE_LONG_TERM describes a preset where Teleport will members are granted roles
  // that grants long term access to resources.
  PRESET_TYPE_LONG_TERM = 1;
  // PRESET_TYPE_SHORT_TERM describes a preset where members are granted
  // roles that require them to request for access and upon approval gain short
  // term access.
  PRESET_TYPE_SHORT_TERM = 2;
}
```

##### PRESET_TYPE_SHORT_TERM

Teleport will create the following type of roles and create/modify the access list accordingly:

- `access`: Role(s) related to allowing access to resources. The access specs are determined by admin input. These roles are directly assigned to members of this access list (as member grants).
- `requester`: A role that allows requesting for the resources defined in the `access` roles. This role is not automatically assigned to any user and is a role an admin can optionally assign to any user not assigned as a member to this access list.
- `reviewer`: A role that allows reviewing access requests to resources defined in the "access" roles. This role is directly assigned to owners of this access list (as owner grants).

##### PRESET_TYPE_LONG_TERM

Teleport will create the following type of roles and create/modify the access list accordingly:

- `access`: Role(s) related to allowing access to resources. The access specs are determined by admin input. These roles are NOT directly assigned to anyone but instead are indirectly assigned to other roles created below.
- `requester`: A role that allows requesting for the resources defined in the `access` roles. This role is directly assigned to members of this access list (as member grants).
- `reviewer`: A role that allows reviewing access requests to resources defined in the `access` roles. This role is directly assigned to owners of this access list (as owner grants).

##### Labeling access list with preset type used

Access lists created with a preset will be `labeled` with the type of preset requested in the format: `teleport.internal/access-list-preset: <long-term | short-term>`

The label helps the web UI detect access lists created with a preset and which preset was used.

#### List of role specs defining the access to resources

The roles that Teleport will create for an access list will depend on the role specs defined by an admin and forwarded to from FE to Proxy in the `UIAccessListWithPresetRequest` request:

```go
type UIAccessListWithPresetRequest struct {
  ...
  // AccessRoles defines the role specifications for resource access
  AccessRoles []types.Role `json:"accessRoles,omitempty"`
}
```

##### Role naming format

In order to ensure uniqueness and help identity what roles belong to an access list created with a preset, the naming convention takes the following format:

`<purpose>-acl-preset-<access list metadata name (UID)>`

| Parts                             |                             Explanation                             |                     Example Values |
| :-------------------------------- | :-----------------------------------------------------------------: | ---------------------------------: |
| \<purpose\>                       | short word that describes the purpose of role, controlled by client | requester, reviewer, access, awsic |
| acl-preset                        |                   stands for "access list preset"                   |                                n/a |
| <access list metadata name (UID)> |        helps identify which access list this role belongs to        |                                n/a |

For example, if an access list has metadata.name `18903baf-xXXx-xxXX-xxxx-92f81a5432ca` then the role name examples are:

- A role that allows requesting access: `requester-acl-preset-18903baf-xXXx-xxXX-xxxx-92f81a5432ca`
- A role that allows reviewing access: `reviewer-acl-preset-18903baf-xXXx-xxXX-xxxx-92f81a5432ca`
- A role that defines access to resources: `access-acl-preset-18903baf-xXXx-xxXX-xxxx-92f81a5432ca`
- A role that defines access specific to AWS IC apps: `awsic-acl-preset-18903baf-xXXx-xxXX-xxxx-92f81a5432ca`

##### Defining multiple role specs

Creating multiple roles is supported to allow flexibility in defining access to resources.

Some resources, like the application resource sub_kind `aws_ic_account`, the labels are not controlled by users. And to define access to AWS IC apps, it requires defining both `app_labels` and `account_assignments`.

If only one role was supported, `app_labels` will either define access to AWS IC apps or non AWS IC apps since multiple label is an `AND` operation. To allow both types of applications, two roles can be created that allows access to both types of apps:

```yaml
# role spec 1
# allows all applications with labels "env: staging"
spec:
  allow:
    app_labels:
      'env': 'staging'
```

{/_ spell-checker: disable _/}

```yaml
# role spec 2
# allows all applications with labels "teleport.dev/origin: aws-identity-center"
# and allows signing in with ARN XXXX
spec:
  allow:
    app_labels:
      'teleport.dev/origin': 'aws-identity-center'
    account_assignments:
      - account: '1234-AWS-Account-ID'
        permission_set: arn:aws:sso:::permissionSet/ssoins-XXXX
```

{/_ spell-checker: enable _/}

#### Request Backend Handling Flow

{/_ spell-checker: disable _/}

The frontend sends the preset type, access list configuration, and role specifications to the web endpoints (`POST /enterprise/accesslistpreset` for creation or `PUT /enterprise/accesslistpreset/:accessListId` for updates). The proxy implements these endpoints as simple wrappers that orchestrate two sequential gRPC calls to the auth server: first calling `AtomicWriteRoles/ModifyRolesForAccessList` to create or update the requester, reviewer, and access roles atomically with strong consistency guarantees that prevent concurrent role updates, then creating or updating the access list grants referencing the created roles.

{/_ spell-checker: enable _/}

The flow is kept in the proxy rather than adding a dedicated access list gRPC endpoint because this feature is a simple UX wrapper designed exclusively for the frontend flow. The orchestration logic is straightforward and does not require server-side implementation in the auth layer. By implementing it as a proxy wrapper, the design remains lightweight and avoids adding complexity to the core access list gRPC API for a feature that is solely used by FE

This two-phase approach keeps role orchestration in the proxy layer while ensuring atomicity. The Access List preset feature is a strict UX/UI flow not supported by CLI or Terraform, as it can be abstracted via a Terraform module with automatic role creation. Terraform manages partial state and allows destroy/retry operations on failure, whereas the frontend flow must be atomic to avoid partial backend state and complex rollback mechanisms.

#### Deleting access lists created with a preset

The same [existing endpoint](https://github.com/gravitational/teleport/blob/e26f1f01a0a2433ae104f01ada73a1bf9b935963/api/proto/teleport/accesslist/v1/accesslist_service.proto#L43) will be used to delete an access list with a preset.

In the web UI after an access list has successfully deleted, a popup dialogue will render letting users know that they can optionally delete the roles related to the deleted access list. A row of related roles will be rendered, with a delete button for each row.

Since deleting a role that is used (e.g. a role assigned in another role) can lock a user out with "role not found" error, a warning banner will also be rendered on the delete dialogue to let users know that it is their responsibility to ensure that the roles that they are about to delete is unused.

### Product Metrics

To help gain insight into how this wizard is used we will implement the following product metrics:

```proto
// AccessListStatus represents a access list wizard step outcome.
enum AccessListStatus {
  ACCESS_LIST_STATUS_UNSPECIFIED = 0;
  // The user tried to complete the action and it succeeded.
  // e.g. going to next step,
  ACCESS_LIST_STATUS_SUCCESS = 1;
  // The user skipped the action (some steps may be optional).
  ACCESS_LIST_STATUS_SKIPPED = 2;
  // The user tried to complete the action and it failed.
  ACCESS_LIST_STATUS_ERROR = 3;
  // The user did not complete the action and left the wizard.
  ACCESS_LIST_STATUS_ABORTED = 4;
}

// AccessListPreset represents the access list preset type.
enum AccessListPreset {
  ACCESS_LIST_PRESET_UNSPECIFIED = 0;
  ACCESS_LIST_PRESET_SHORT_TERM = 1;
  ACCESS_LIST_PRESET_LONG_TERM = 2;
}

// AccessListIntent describes what the user intent is.
enum AccessListIntent {
  ACCESS_LIST_INTENT_UNSPECIFIED = 0;
  // User wants to update an access list.
  ACCESS_LIST_INTENT_PRESET_UPDATE = 1;
  // User wants to create a new access list.
  ACCESS_LIST_INTENT_PRESET_CREATE = 2;
}

// AccessListStepStatus contains fields that track a particular step outcome.
message AccessListStepStatus {
  // Indicates the step outcome.
  AccessListStatus status = 1;
  // Contains error details in case of Error Status.
  string error = 2;
}

// AccessListMetadata contains common metadata for access list related events.
message AccessListMetadata {
  // Uniquely identifies access list wizard "session". Will allow to correlate
  // events within the same access list wizard run.
  string id = 1;
  // anonymized
  string user_name = 2;
  // Describes the sessions preset.
  AccessListPreset preset = 3;
  // Describes the sessions intent.
  AccessListIntent intent = 4;
}

// UIAccessListDefineAccessEvent is emitted when user is finished with the step
// that defines access to resources.
message UIAccessListDefineAccessEvent {
  AccessListMetadata metadata = 1;
  AccessListStepStatus status = 2;
}

// UIAccessListDefineIdentitiesEvent is emitted when user is finished with the
// step that defines resource identities/principals.
message UIAccessListDefineIdentitiesEvent {
  AccessListMetadata metadata = 1;
  AccessListStepStatus status = 2;
}

// UIAccessListBasicInfoEvent is emitted when user is finished with the step
// that defines basic info of an access list (title, desc, etc).
message UIAccessListBasicInfoEvent {
  AccessListMetadata metadata = 1;
  AccessListStepStatus status = 2;
}

// UIAccessListDefineMembersEvent is emitted when user is finished with the
// step that defines access list members.
message UIAccessListDefineMembersEvent {
  AccessListMetadata metadata = 1;
  AccessListStepStatus status = 2;
}

// UIAccessListDefineOwnersEvent is emitted when user is finished with the
// step that defines access list owners.
message UIAccessListDefineOwnersEvent {
  AccessListMetadata metadata = 1;
  AccessListStepStatus status = 2;
}

```

To track success or failure of API calls:

```proto
// UIAccessListCreatedEvent is emitted when an access list is created
// (or errors out).
message UIAccessListCreatedEvent {
  AccessListMetadata metadata = 1;
  AccessListStepStatus status = 2;
}

// UIAccessListCreatedEvent is emitted when an access list is updated
// (or errors out).
message UIAccessListUpdatedEvent {
  AccessListMetadata metadata = 1;
  AccessListStepStatus status = 2;
}
```

To track how long the entire wizard took:

```proto
// UIAccessListStartedEvent is emitted when user starts the wizard
// (after selection of a wizard).
message UIAccessListStartedEvent {
  AccessListMetadata metadata = 1;
  AccessListStepStatus status = 2;
}


// UIAccessListCompletedEvent is emitted when user completes a wizard.
message UIAccessListCompletedEvent {
  AccessListMetadata metadata = 1;
  AccessListStepStatus status = 2;
}
```

When user leaves to integrate with Okta (by clicking on a CTA button during the wizard) or when user is coming from Okta integration:

```proto
// AccessListIntegrate describes what integration user
// was interested in.
enum AccessListIntegrate {
  ACCESS_LIST_INTEGRATE_UNSPECIFIED = 0;
  // User wants to integrate Okta or is coming from Okta.
  ACCESS_LIST_INTEGRATE_OKTA = 1;
}

// UIAccessListIntegrateOktaEvent is emitted when a user leaves the wizard
// to enroll an integration by clicking on a CTA button in the wizard.
message UIAccessListIntegrateEvent {
  AccessListMetadata metadata = 1;
  AccessListIntegrate integrate = 2;
}

// UIAccessListFromOktaEvent is emitted when a user comes from a
// integration by clicking on any of the set up access CTA buttons
// (e.g: status page, okta overview page, or finished step during integration).
message UIAccessListFromIntegrationEvent {
  AccessListMetadata metadata = 1;
  AccessListIntegrate integrate = 2;
}
```

### Optimization Consideration

For each access list created with a preset, about 3-4 roles are created for it. This can result in hundreds of roles related to access lists.

A possible solution to reduce the roles down to 1-2 per access list is to create templated roles meant for `requesters` and `reviewers` where we use `claims_to_roles` fields that allows dynamic mapping from claims (traits) to roles.

Currently, the role resource only supports `claims_to_roles` for the following:

- `request.claims_to_roles -> request.roles` which dynamically determines which roles a user can request access to
- `review_requests.claims_to_roles -> review_requests.roles` which dynamically determines which requested roles a user can review

Example use of `review_requests.claims_to_roles`, where a trait `access-list-template: access, editor` allows the user to review requests for `access` and `editor` role:

```yaml
kind: role
metadata:
  name: access-list-reviewer-template
spec:
  allow:
    review_requests:
      claims_to_roles:
        # any trait key starting with "access-list-template"
        - claim: access-list-template
          # with any trait values (a valid role name)
          value: '*'
          # gets assigned those trait values (roles)
          roles:
            - $1
version: v8
```

To extend this feature to access list, we will need a similar `claims_to_roles` support for the following role fields:

- `request.search_as_roles` required to allow user to list + search for resources to request
- `review_requests.preview_as_roles` allows the reviewer to preview details of an access request (e.g. view friendly names of resources instead of its UID that are usually just random alphanumerics)
