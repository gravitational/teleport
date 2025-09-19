---
author: Lisa Kim (lisa@goteleport.com)
state: draft
---

# RFD XXXX - Templated Access Lists

# Required Approvers

- Engineering: @r0mant || @smallinsky (marek) && @kopiczko (pavel)
- Product: @r0mant

## What

Access lists grant roles (and traits) to users on a long lived basis. Currently, to grant access through an access list, admins must select roles that already exists. This requires the admins to know what a role is and know how to create a role to customize access.

This RFD proposes a new type of access list, a templated access list that removes the need for admins to know about roles. Templated access lists will take a simplified access specification and execute system behaviors such as Teleport creating the required roles and then Teleport assigning those roles to members and owners upon creating an Access List.

## Why

Improves access list usability especially for day one users. Templated access list allows an admin to focus on users and what resources users should have access to. It removes the need for an admin to learn how to create roles and removes how roles have relation to an access list because Teleport will do it for them.

### User story: As an admin, I want to create an access list that require members to request for short-term access to selected resources

The template type to use for this case is `short-term`.

Template type `short-term` represents an access list that utilizes JIT. Owners are reviewers. Members are requesters that are required to request access to resources and then upon approval are granted short-term access to requested Teleport resources.

Admin will define what resources members will have access to by specifying the resource fields that control their access (e.g: labels).

### User story: As an admin, I want to create an accesss list that grants long-term access to resources for members

The template type to use for this case is `long-term`.

Template type `long-term` represents an access list that grants members standing access to Teleport resources. Owners will have no special purpose other than to audit.

Admin will define what resources members will have access to by specifying the resource fields that control their access (e.g: labels).

This type of template is similar to how access list works now (non integrated types). Only difference is Teleport will create the necessary role for the admin.

### Data Model

#### Type Field

In the current [AccessListSpec](https://github.com/gravitational/teleport/blob/bbb0f46b22ff88299908bef8dcf85d292aa379e1/api/proto/teleport/accesslist/v1/accesslist.proto#L75) model, there exists a field called `type`. We will introduce an additional type called `templated` to indicate an access list used a template upon creation.

```proto
message AccessListSpec {
  // ... other existing fields

  // Existing type values: "" (default), "scim", and "static"
  // NEW value: "templated"
  string type = 12;
}
```

#### Template Specification

A new field `template_config` will be introduced in the current [AccessListSpec](https://github.com/gravitational/teleport/blob/bbb0f46b22ff88299908bef8dcf85d292aa379e1/api/proto/teleport/accesslist/v1/accesslist.proto#L75) model.

```proto
message AccessListSpec {
  // ... other existing fields

  AccessListTemplateConfig template_config = 13;
}

// AccessListTemplateConfig describes the template used.
message AccessListTemplateConfig {
  // template type where values can be "short_term" or "long_term"
  string type = 1;
  // allow is the set of conditions evaluated to grant access.
  AccessConditions allow = 2;
}
```

The field `AccessConditions` is really just a copy and paste of the existing type [RoleConditions](https://github.com/gravitational/teleport/blob/31143cca86ec73d7404e5f90044996eafff199c8/api/proto/teleport/legacy/types/types.proto#L3759) but only takes the fields that is relevant to an access list and molds it to an organized structure so it will be easier to define with `tctl` users.

The `AccessConditions` model describes fields that control Teleport resource access.

```proto
// AccessConditions defines the access to different resources.
message AccessConditions {
  ApplicationAccess application = 1;
  AWSIdentityCenter aws_identity_center = 2;
  DatabaseAccess database = 3;
  GitServerAccess git_server = 4;
  KubernetesAccess kubernetes = 5;
  ServerAccess server = 6;
  WindowsDesktopAccess windows_desktop = 7;
}

// ApplicationAccess are access related fields for application resource.
message ApplicationAccess {
  repeated teleport.label.v1.Label labels = 1;
  repeated string aws_role_arns = 2;
  repeated string azure_identities = 3;
  repeated string gcp_service_accounts = 4;
  types.MCPPermissions mcp = 5;
}

// AWSIdentityCenter are access related fields for AWS identity center.
// AWS identity center is part of application access but since its
// labels can only be set by Teleport and are very specific to AWS
// identity center it doesn't allow including other application labels.
// Having it's own field allows flexibility (e.g: creating a role
// specific to just AWS identity center).
message AWSIdentityCenter {
  repeated teleport.label.v1.Label labels = 1;
  types.IdentityCenterAccountAssignment account_assignments = 2;
}

// DatabaseAccess are access related fields for db resource.
message DatabaseAccess {
  repeated teleport.label.v1.Label labels = 1;
  repeated string names = 2;
  repeated string users = 3;
}

// GitServerAccess are access related fields for git server resource.
message GitServerAccess {
  repeated types.GitHubPermission permissions = 1;
}

// KubernetesAccess are access related fields for kube resource.
message KubernetesAccess {
  repeated teleport.label.v1.Label labels = 1;
  repeated string groups = 2;
  repeated string users = 3;
  repeated types.KubernetesResource resources = 4;
}

// ServerAccess are access related fields for server resource.
message ServerAccess {
  repeated teleport.label.v1.Label labels = 1;
  repeated string logins = 2;
}

// WindowsDesktopAccess are access related fields for windows desktop resource.
message WindowsDesktopAccess {
  repeated teleport.label.v1.Label labels = 1;
  repeated string logins = 2;
}
```

### UX

For all CLI examples, the template feature is added to the existing access list field `spec`. For `spec.title` new value `templated` is used. And a new field `spec.template_config` is introduced:

#### tctl example

```yaml
version: v1
kind: access_list
metadata:
  name: example-long-term-template
spec:
  title: "Example Long-Term Template"
  type: "templated"
  template_config:
    type: long_term
    allow:
      application:
        labels:
          env:
          - prod
          - staging
        aws_role_arns:
        - some-arn
      server:
        labels:
          env:
          - dev
        logins:
        - ubuntu
        - ec2-user
  ...
```

#### Terraform example

```hcl
resource "teleport_access_list" "example-long-term-template" {
  header =  {
    version = "v1"
    metadata = {
      name = "example-long-term-template"
    }
  }

  spec = {
    title = "Example Long-Term Template"
    type = "templated"
    template_config = {
      type = "long_term"
      allow = {
        application = {
          labels = [
            {
              name = "env",
              values = ["prod"]
            },
            {
              name = "env",
              values = ["staging"]
            },
          ]
          aws_role_arns = ["some-arn"]
        }
        server = {
          labels = [{
            name = "env",
            values = ["dev"]
          }]
          logins = ["ubuntu", "ec2-user"]
        }
        ...
      }
    }
   ...
  }
}
```

#### Web application

The web UI/UX has already gone through POC iterations and has been approved by sasha and roman. Designs are getting finalized in [figma](https://www.figma.com/design/CzxaBHF8hirrYv2bTVa4Rw/Identity-Governance?node-id=5590-174098&p=f&t=mCDqqdU2YrUhaBkW-0).

### Implementation

#### Create

A templated access list can only be created if access list field `type: templated` and `template_config.type: long_term | short_term` is defined. The field `template_config.allow` will not be required in case the user wants to define the access later or remove access altogether.

If `template_config.allow` is defined, Teleport will take the template specifications and create appropriate roles and then assign them as member/owner grants.

If `template_config.allow` was once defined, but becomes undefined, Teleport will unassign grants and delete all roles related to this access list.

All roles created by Teleport will be labeled with

- `teleport.internal/resource-type: system`: Tags this role as Teleport managed. Teleport also supports filtering out system managed roles when [listing roles](https://github.com/gravitational/teleport/blob/a4eb5edab6f91a9373857f2449d1bcaf1a2a9d18/api/proto/teleport/legacy/types/types.proto#L3436).
- `teleport.internal/access-list-name: <UID of access list>`: Tags this role as belonging to an access list. This label will be used to prevent users from updating this role. Teleport currently does not prevent users from modifying system marked roles with `tctl`. This presents a problem where the `access` definition on a templated access list is not in sync with the actual role resource. Rejecting updates will handle this issue.

##### System-managed roles for short-term template

- access: a role that defines access to general resources - this role is not assigned directly to anyone
- access-aws-ic: a role that defines access specific to AWS identity center - this role is not assigned directly to anyone
- reviewer: a role assigned to owner grants that allow them to review requests to resources defined in `access*` role
- requester: a role assigned to member grants that allow them to search and request for resources defined in `access*` role

##### System-managed roles for long-term template

- access: a role that defines access to general resources - assigned to member grants
- access-aws-ic: a role that defines access specific to AWS identity center - assigned to member grants

##### Naming format for the system-managed roles

In order to ensure uniqueness, the naming convention takes the following format:

`templated-acl-<purpose>-role-<UID of access list>`

| Parts                 |                   Explanation                    |                             Example Values |
| :-------------------- | :----------------------------------------------: | -----------------------------------------: |
| templated-acl         |        stands for "templated access list"        |                                        n/a |
| \<purpose>-role       |  short word that describes the purpose of role   | requester, reviewer, access, access-aws-ic |
| \<UID of access list> | describes which access list this role belongs to |                                        n/a |

Example names of system-managed roles, if an access list with metadata.name is `b037d55d-c076-474e-b4a8-bc4d0f10f19d`

- templated-acl-requester-role-b037d55d-c076-474e-b4a8-bc4d0f10f19d
- templated-acl-reviewer-role-b037d55d-c076-474e-b4a8-bc4d0f10f19d
- templated-acl-access-role-b037d55d-c076-474e-b4a8-bc4d0f10f19d
- templated-acl-access-aws-ic-role-b037d55d-c076-474e-b4a8-bc4d0f10f19d

#### Update

`spec.type: templated`, `template_config.type`, member grant, and owner grant fields will not be modifiable.

Only the `template_config.allow` is modifiable.

Any updates made on a templated access list will upsert roles regardless if access has actually changed. This is in part to keep the roles in sync. When updating roles, the previous version of the role will be maintained to prevent unintended role behavioral changes with different role verisons. If a user wants to upgrade to a newer role version, user must create a new access list, which will use the latest role version.

When modifying, Teleport will also delete orphaned roles if present. A role can become orphaned if a user changes access in a way it changes grants. E.g: if a user removes access for AWS IC, the role created specifically for AWS IC access becomes unused.

#### Delete

In the backend, after an access list is successfully deleted, all roles tied to that deleted access list will also be deleted. Note that the access list must be deleted first as roles assigned to access list cannot be deleted.

##### Reconciler that cleans up orphaned roles

If deleting roles fail for some reason, it leaves behind orphaned roles. A delete reconciler will be created where periodically the reconciler will iterate through all the roles, and if a role belonged to an access list and that access list no longer exists, reconciler will delete the role.

### Backwards compatibility

#### Web application

A v2 endpoint for creating and updating access lists will be introduced so the web app can reject requests going to older proxies and help notify user that all proxies must be upgraded to the version that this feature will be released in.

#### Downgrading cluster

If a templated access list is created in a cluster that supports this feature, and cluster is downgraded to a version not supporting this feature, the access list will essentially behave like a regular access list. Users will lose the ability to modify access through access list. And upon delete, the system-managed roles will remain behind.

Upon upgrading again to a cluster that supports this feature:

- If the access list still exists, it should work as before. There is a chance a user could've modified this role (tctl). The best we can do here is to document that system roles should not be modified. Also, any updates made will upsert the roles again which will bring it back in sync.
- If the access list does not exist, the reconciler will delete the orphaned roles.

### Feature extension

Possibly add more option to `short-term` template such as providing the option to control requester role fields defined below:

```proto
message TemplateShortTerm {
  AccessConditions allow = 1;

  // NEW field (didn't think too much on the naming)
  // Allows admin to fine tune fields related to the requester role.
  RequesterConditions requester_allow= 2;
}

// Controls related to access requests.
message RequesterConditions {
  google.protobuf.Timestamp max_duration = 1;
  AccessRequestConditionsReason reason = 2;
  repeated string suggested_reviewers = 3;
  repeated AccessReviewThreshold thresholds = 4;
}
```
