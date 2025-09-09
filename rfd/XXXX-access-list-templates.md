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

// TemplateRoleMetadata contains read-only fields
// and is only set and updated by Teleport. Used
// by reconciler to revert roles back to default
// if changes were detected.
message TemplateRoleMetadata {
  // revision stores the last revision made by Teleport
  // on a role.
  string revision = 1;
  // initial_version preserves the initial version of role
  // to prevent unintended role behavioral change with
  // newer role versions or when user has changed the role
  // version.
  string initial_version = 2;
}

// TemplateLongTerm describes fields required to create
// an access list with long term access grant.
message TemplateLongTerm {
    // access_condition defines access to resources
    // and its principals.
    AccessConditions allow = 1;
    TemplateRoleMetadata role_access = 2;
}

// TemplateShortTerm describes fields required to create
// an access list with short term access grant.
message TemplateShortTerm {
    // access_condition defines access to resources
    // and its principals.
    AccessConditions allow = 1;
    TemplateRoleMetadata role_access = 2;
    TemplateRoleMetadata role_requester = 3;
    TemplateRoleMetadata role_reviewer = 4;
}
```

The field `AccessConditions` is really just a copy and paste of the existing type [RoleConditions](https://github.com/gravitational/teleport/blob/31143cca86ec73d7404e5f90044996eafff199c8/api/proto/teleport/legacy/types/types.proto#L3759) but only takes the fields that is relevant to an access list and molds it to an organized structure so it will be easier to define with `tctl` users.

The `AccessConditions` model describes access to resources by its labels and relevant resource principals.

```proto
// AccessConditions defines the access to different resources.
message AccessConditions {
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

### UX

For all CLI examples, the template feature is added to the existing access list field `spec`. For `spec.title` new value `template` is used. And a new field `spec.template_config` is introduced:

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

#### Web application

The web UI/UX has already gone through POC iterations and has been approved by sasha and roman. Designs are getting finalized in [figma](https://www.figma.com/design/CzxaBHF8hirrYv2bTVa4Rw/Identity-Governance?node-id=5590-174098&p=f&t=mCDqqdU2YrUhaBkW-0).

### Implementation

#### Create

A templated access list can only be created if access list fields `type: template` and `template_config` defines at least one resource field with its labels set. Resource principals will not be required.

Once these fields are validated, Teleport will take the template specifications and create appropriate roles and then assign them to member and owner.

All roles created by Teleport will be labeled with `teleport.internal/resource-type: system` and will be referred as system-managed roles.

##### System-managed roles for short-term template

- access: a role that defines the access to resources - this role is not assigned directly to anyone
- reviewer: a role assigned to owner grants that allow them to review requests to resources defined in `access` role
- requester: a role assigned to member grants that allow them to search and request for resources defined in `access` role

##### System-managed roles for long-term template

- access: a role that defines the access to resources and is assigned to member grants

##### Naming format for the system-managed roles

In order to ensure uniqueness and help identifying which roles belong to access lists, the naming convention takes the following format:

`<purpose>-acl-template-<access list metadata name (UID)>`

| Parts                             |                      Explanation                      |              Example Values |
| :-------------------------------- | :---------------------------------------------------: | --------------------------: |
| \<purpose\>                       |     short word that describes the purpose of role     | requester, reviewer, access |
| acl-template                      |           stands for "access list template"           |                         n/a |
| <access list metadata name (UID)> | helps identify which access list this role belongs to |                         n/a |

Example names of system-managed roles, if an access list with metadata.name is set to `abcd1234`

- requester-acl-template-abcd1234
- reviewer-acl-template-abcd1234
- access-acl-template-abcd1234

#### Role reconciler for the system-managed roles

Teleport does not prevent users from modifying system-managed roles. A user is able to `tctl create existing-role.yaml -f`. This poses a problem where the `access` definition defined on an access list might not be in sync with the actual role resource (and its other related roles). To keep roles in sync with access lists, a reconciler will be created that will periodically (or on watched role event) will read roles related to access list and query for its matching access list and match the `role.metadata.revision` with `accesslist.spec.template_config.short_term|long_term.role_XXX_revision`. On mismatch, the role will essentially be re-created and upserted, and once successful the `revision` field on the access list will also be updated.

When roles are `re-created`, the saved `initial_role` version will be used to prevent unintended role behavioral changes with newer roles (or the version the user manually has set it to).

The reconciler will attempt to immediately reconcile on startup to handle the edge case of modifying roles while offline.

The reconciler will also clean up stale roles by deleting them if querying for its related access list fails with a `Not Found` error.

#### Update

The `type: template`, member grant, and owner grant will not be modifiable.

`template_config` field is partly modifiable. The field that defines the access to resources can be modified. The `oneof type` will not be modifiable eg: if an access list template was originally `short-term`, a user cannot change the template to `long-term`.

For both templates, since only the `access` part of the `template_config` can be modified, the system-managed role related to `access` will be updated.

#### Delete

In the backend, after an access list is successfully deleted, all system-managed roles tied to that deleted access list will also be deleted. Note that the access list must be deleted first as roles assigned to access list cannot be deleted.

In the case deleting roles fail for some reason, the errors can be ignored as the reconciler will eventually clean up stale roles.

### Backwards compatibility

#### Web application

A v2 endpoint for creating and updating access lists will be introduced so the web app can reject requests going to older proxies and help notify user that all proxies must be upgraded to the version that this feature will be released in.

#### Downgrading cluster

If a templated access list is created in a cluster that supports this feature, and cluster is downgraded to a version not supporting this feature, the access list will essentially behave like a regular access list. Users will lose the ability to modify access through access list. And upon delete, the system-managed roles will remain behind.

Upon upgrading again to a cluster that supports this feature:

- if the access list still exists, the reconciler will resync the roles with the access defined in the access list
- if the access list does not exist, the reconciler will delete the stale roles

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
