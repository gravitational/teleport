---
author: Lisa Kim (lisa@goteleport.com)
state: draft
---

# RFD 222 - Templated Access Lists

# Required Approvers

- Engineering: @r0mant || @smallinsky (marek) && @kopiczko (pavel)
- Product: @r0mant

## What

Access lists grant roles (and traits) to users on a long lived basis. Currently, to grant access through an access list, admins must select roles that already exists. This requires the admins to know what a role is and know how to create a role to customize access.

This RFD proposes a new type of access list, a templated access list that removes the need for admins to know about roles. Templated access lists will take a simplified access specification and execute system behaviors such as Teleport creating the required roles and then Teleport assigning those roles to members and owners upon creating an Access List.

## Why

Improves Access List usability especially for day one users. Templated Access List allows an admin to focus on users and what resources users should have access to. It removes the need for an admin to learn how to create roles and removes how roles have relation to an Access List because Teleport will do it for them.

### User story: As an admin, I want to create an access list that require members to request for short-term access to selected resources

The template type to use for this case is `short-term`.

Template type `short-term` represents an access list that utilizes JIT. Owners are reviewers. Members are requesters that are required to request access to resources and then upon approval are granted short-term access to requested Teleport resources.

Admin will define what resources members will have access to by specifying the resource kinds and their labels and their resource principals.

### User story: As an admin, I want to create an accesss list that grants long-term access to resources for members

The template type to use for this case is `long-term`.

Template type `long-term` represents an access list that grants members standing access to Teleport resources. Owners will have no special purpose other than to audit.

Admin will define what resources members will have access to by specifying the resource kinds and their labels and their resource principals.

This type of template is similar to how access list works now (non integrated types). Only difference is Teleport will create the necessary role for the admin.

### Data Model

#### Type Field

In the current [AccessListSpec](https://github.com/gravitational/teleport/blob/bbb0f46b22ff88299908bef8dcf85d292aa379e1/api/proto/teleport/accesslist/v1/accesslist.proto#L75) model, there exists a field called `type`. We will introduce an additional type called `template` to indicate an access list used a template upon creation.

```proto
message AccessListSpec {
  // ... other existing fields

  // Existing type values: "" (default), "scim", and "static"
  // NEW value: "template"
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
  oneof template {
    TemplateLongTerm long_term = 1;
    TemplateShortTerm short_term = 2;
  }
}

// TemplateLongTerm describes fields required to create
// an access list with long term access grant.
message TemplateLongTerm {
  // access_condition defines access to resources
  // and its principals.
    AccessConditions allow = 1;
}

// TemplateShortTerm describes fields required to create
// an access list with short term access grant.
message TemplateShortTerm {
  // access_condition defines access to resources
  // and its principals.
    AccessConditions allow = 1;
}
```

The field `AllowResourceAccessConditions` is really just a copy and paste of the existing type [RoleConditions](https://github.com/gravitational/teleport/blob/31143cca86ec73d7404e5f90044996eafff199c8/api/proto/teleport/legacy/types/types.proto#L3759) but only takes the fields that is relevant to an access list and molds it to an organized structure so it will be easier to define with `tctl` users.

The `AllowResourceAccessConditions` model describes access to resources by its labels and relevant resource principals.

```proto
// AllowResourceAccessConditions defines the access to different resources.
message AllowResourceAccessConditions {
  ApplicationAccess application = 1;
  DatabaseAccess database = 2;
  GitServerAccess git_server = 3;
  KubernetesAccess kubernetes = 4;
  ServerAccess server = 5;
  WindowsDesktopAccess windows_desktop = 6;
}

// ApplicationAccess are access related fields for application resource.
message ApplicationAccess {
  repeated teleport.label.v1.Label labels = 1;
  repeated string aws_role_arns = 2;
  repeated string azure_identities = 3;
  repeated string gcp_service_accounts = 4;
  types.MCPPermissions mcp = 5;
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

### CLI UX

For all CLI examples, feature is added to the existing access list field `spec`. For `spec.title` new value `template` is introduced. And a new field `spec.template_config` is added:

#### tctl example

```yaml
version: v1
kind: access_list
metadata:
  name: example-long-term-template
spec:
  title: "Example Long-Term Template"
  type: "template"
  template_config:
    long_term:
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
    type = "template"
    template_config = {
      long_term = {
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
    }
   ...
  }
}
```

### Implementation

#### Create

A templated access list can only be created if access list fields `type: template` and `template_config` defines at least one resource field with its labels set. Resource principals will not be required.

Once these fields are validated, Teleport will take the template specifications and create system roles and then assign them to member and owner.

##### System roles for short-term template

- access: a role that defines the access to resources - this role is not assigned directly to anyone
- reviewer: a role assigned to owner grants that allow them to review requests to resources defined in `access` role
- requester: a role assigned to member grants that allow them to search and request for resources defined in `access` role

##### System roles for long-term template

- access: a role that defines the access to resources and is assigned to member grants

##### Naming system roles

In order to ensure uniqueness and help identifying which roles belong to access lists, the naming convention takes the following format:

`<purpose>-acl-role-<access list metadata name (UID)>`

| Parts                             |                      Explanation                      |              Example Values |
| :-------------------------------- | :---------------------------------------------------: | --------------------------: |
| \<purpose\>                       |     short word that describes the purpose of role     | requester, reviewer, access |
| acl-role                          |             stands for "access list role"             |                         n/a |
| <access list metadata name (UID)> | helps identify which access list this role belongs to |                         n/a |

Example names of system roles, if an access list with metadata.name is set to `abcd1234`

- requester-acl-role-abcd1234
- reviewer-acl-role-abcd1234
- access-acl-role-abcd1234

#### Update

The `type: template`, member grant, and owner grant will not be modifiable.

Minus the fields that are already modifiable, `template_config` field is partly modifiable. The field that defines access to resources can be modified. The `oneof type` will not be modifiable eg: if an access list template was originally `short-term`, a user cannot change the template to `long-term`.

For both templates, since only the `access` part of the `template_config` can be modified, the system role related to `access` will be updated.

##### Update quirk

There is a quirk where the `access` definition set on an access list might not be in sync with the actual role resource because we don't prevent users from editing system roles with `tctl`. If such a case happens, ultimately the role resource is the source of truth. Any updates made to `template_config` will overwrite any previous edits directly made to the system roles.

#### Delete

In the backend, after an access list is successfully deleted, all system roles tied to that deleted access list will also be deleted.

In the case deleting roles fail for some reason, we can offer a retry if we detect the failure was due to clean up. An API endpoint will be created that is specific to cleaning up templated access list (which is to just delete roles at this moment).

### Feature extension

Possibly add more option to `short-term` template such as providing the option to control requester role fields defined below:

```proto
message TemplateShortTerm {
  AllowResourceAccessConditions access_condition = 1;

  // NEW field (didn't think too much on the naming)
  // Allows admin to fine tune fields related to the requester role.
  RequesterCondition requester_condition= 2;
}

// Controls related to access requests.
message RequesterCondition {
  google.protobuf.Timestamp max_duration = 1;
  AccessRequestConditionsReason reason = 2;
  repeated string suggested_reviewers = 3;
  repeated AccessReviewThreshold thresholds = 4;
}
```
