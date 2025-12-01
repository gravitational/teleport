---
author: Lisa Kim (lisa@goteleport.com)
state: draft
---

# RFD 0236 - Access Lists Created with Presets

# Required Approvers

- Engineering: @r0mant || @smallinsky (marek) && @kopiczko (pavel)
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

All admin needs to do for this preset is to define the metadata, owners, and members of the access list and define the access to resources (e.g. node_labels and node logins) and Teleport will create the following type of roles and create/modify the access list accordingly:

- `access`: Role(s) related to allowing access to resources. The access specs are determined by admin input. These roles are directly assigned to members of this access list (as member grants).
- `requester`: A role that allows requesting for the resources defined in the `access` roles. This role is not automatically assigned to any user and is a role an admin can optionally assign to any user not assigned as a member to this access list.
- `reviewer`: A role that allows reviewing access requests to resources defined in the "access" roles. This role is directly assigned to owners of this access list (as owner grants).

### User story: As an admin, I want to require a select group of users (local or Okta imported) to request for short-term access to resources

The admin can create an access list using the preset `short-term`.

Preset `short-term` represents an access list that utilizes JIT. Owners of the access list are reviewers. Members of the access list are requesters that are required to request access to resources and then upon approval are granted short-term access to requested Teleport resources.

All admin needs to do for this preset is to define the metadata, owners, and members of the access list and define the access to resources (e.g. labels) and then Teleport will create the following type of roles and create/modify the access list accordingly:

- `access`: Role(s) related to allowing access to resources. The access specs are determined by admin input. These roles are NOT directly assigned to anyone but instead are indirectly assigned to other roles created below.
- `requester`: A role that allows requesting for the resources defined in the `access` roles. This role is directly assigned to members of this access list (as member grants).
- `reviewer`: A role that allows reviewing access requests to resources defined in the `access` roles. This role is directly assigned to owners of this access list (as owner grants).

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

The dynamic feedback allows admin to essentially double check and preview what members of this access list will see in their own account.

#### Terraform Support

The guide will provide a copyable Terraform script that is equivalent to going through the guide to create an access list with a preset. At the end of the guide, the admin can choose to use the Terraform script only, or let the web app proceed to finish the request.

Similar to dynamic feedback, as the admin is defining/tweaking inputs in the guide, the Terraform script will dynamically update to reflect the latest changes. This script will be visible throughout the entire guide (can optionally be hidden).

The Terraform script will also be available later when viewing the access list created with a preset.

### Security

To prevent users (admins) from overstepping the boundaries set by their assigned roles, the following restrictions are placed when user wants to perform create, delete, or update operations on an access list created with a preset:

- The user is required to have access to perform all CRUD operations on a role. The user may not be directly performing actions on a role, but Teleport will on behalf of the user.
- The [same existing rules](https://github.com/gravitational/teleport.e/blob/e49a5ad654408ce0779622c38c7acda0417bfef0/lib/accesslist/service.go#L1648) are applied when creating/deleting/updating an access list. E.g. for updating an access list the user will have to either be a owner or have access to `access_list` resource rules.
- When defining or modifying access to resources for an access list, the user is limited to what their own roles allow them. For example, lets say a few application resources exists in a Teleport cluster:
  - If the users currently assigned role does not define access to applications (missing `app_labels`) and therefore unable to list applications, the user will not be able to define any access to application.
  - If the users currently assigned role defines a `app_label` giving access to just `some` of the applications, the user will be only able to define access to the resources they can see.
  - If an access list with preset was created, and later a user with lesser permission than the original creator comes in to edit the access to resources, the access definition for resources that produced no results will be removed on save. This prevents defining random labels.
- When modifying access to resources for an access list, the same guide will be provided, unless the web app detected unsupported modifications made on a role meant for an access list. If unsupported modifications (or unsupported role versions) were detected, the web UI will point the user to use the standard role editor instead (how normal access list essentially works). This is because the guide does not support parsing all fields of a role.

### API

A new endpoint will be created to support this feature.

#### Preset type

The new endpoint will expect the type of preset.

Access lists created through this api will be `labeled` with the type of preset requested in the format: `teleport.internal/access-list-preset: <long-term | short-term>`

The label helps the web UI detect access lists created with a preset and which preset was used.

```proto
// PresetType describes what type of preset was requested.
enum PresetType {
  // PRESET_TYPE_UNSPECIFIED is the zero value.
  PRESET_TYPE_UNSPECIFIED = 0;
  // PRESET_TYPE_LONG_TERM describes a preset where members are granted roles
  // that grants long term access to resources.
  PRESET_TYPE_LONG_TERM = 1;
  // PRESET_TYPE_SHORT_TERM describes a preset where members are granted
  // roles that require them to request for access and upon approval gain short
  // term access.
  PRESET_TYPE_SHORT_TERM = 2;
}
```

#### List of role specs defining the access to resources

The new endpoint will accept a list of role specs to upsert.

```proto
// Role describes a role to be upserted by Teleport for an access list.
message Role {
  // name_prefix is the prefix of the role name and allows the client to
  // control part of the role name so clients like the web UI can make some
  // assumptions based on this prefix e.g. what purpose the access role was
  // created for. For the final role name however, Teleport will add a suffix
  // to this name_prefix before upserting the role. The suffix serves to:
  //  - ensure uniqueness by including the access list UID which also
  //    ties this role to that access list making it easier to query.
  //  - add a infix like "acl-preset" that easily identifies that this role
  //    belongs to an access list that was created using a preset.
  //
  // The format of the final role name is:
  //
  // <name_prefix>-acl-preset-<access-list-id>
  //
  // Example:
  // If the acces list ID is "ABCDEF" and name_prefix is "access-primary",
  // then final role name is: `access-primary-acl-preset-ABCDEF`
  string name_prefix = 1;
  // spec defines the access to resources.
  types.RoleSpecV6 spec = 2;
}
```

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

#### Request and response bodies

Similar to existing [UpsertAccessListWithMembers](https://github.com/gravitational/teleport.e/blob/e49a5ad654408ce0779622c38c7acda0417bfef0/lib/accesslist/service.go#L1542) where same logics are used when upserting an access list but with automatic role handling.

```proto
// UpsertAccessListWithPresetRequest is the request for upserting an access
// list with a preset. Using this API will not allow certain access list fields
// to be set or updated:
//  - Member and owner grants: Teleport will always overwrite these fields
//    with the roles Teleport upserted.
message UpsertAccessListWithPresetRequest {
  // preset_type describes what preset type was requested and determines
  // what set of actions Teleport will perform.
  PresetType preset_type = 1;
  // access_roles is a list of role specs that defines the "access" to
  // resources. Teleport will perform role create/update/delete (CUD) per
  // spec.
  //
  // Teleport will delete roles that are NOT in this request. Typically
  // applies when "updating" an access list.
  repeated Role access_roles = 2;
  // access_list is the access list to upsert.
  AccessList access_list = 3;
  // members is the list of access list members to upsert.
  repeated Member members = 4;
}

// UpsertAccessListWithPresetResponse is the response for upserting
// an access list with with a preset.
message UpsertAccessListWithPresetResponse {
  // access_list is the access list that was upserted.
  AccessList access_list = 1;
  // members is the list of access list members that were upserted.
  repeated Member members = 2;
  // roles is a list of all the roles that Teleport upserted.
  repeated types.RoleV6 roles = 3;
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
- `review_requests.preview_as_roles` allows the reviewer to preview details of an access requet (e.g. view friendly names of resources instead of its UID that are usually just random alphanumerics)
