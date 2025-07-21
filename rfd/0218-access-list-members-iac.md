---
authors: Pawel Kopiczko (pawel.kopiczko@goteleport.com)
state: draft
---

# RFD 218 - Access List members IaC

## Required Approvals

* Engineering: @r0mant && @smallinsky
* Product: @r0mant

## What

Ability to manage Access List members with Terraform. This is enterprise-only feature.

## Why

Currently the Access List membership model requires periodic membership reviews. Because of that,
the IaC approach to Access List membership was not provided so far and we don't have a good way to
introduce it for the Access Lists in their current form.

Manual management of Access List membership doesn't always scale. There are ways of proper
structuring teams as Access Lists and then using the nested Access List concept to assign teams to
resources, but that doesn't work when users are managed externally.

The concept of dynamically assigning users to Access Lists (something like membership criteria)
also won't scale when users are managed externally in large organizations. For example Microsoft
Entra ID won't display any groups in SAML assertion [if the user is assigned to more than 150
groups](https://learn.microsoft.com/en-us/entra/identity/hybrid/connect/how-to-connect-fed-group-claims).

A new concept, a ***static Access Lists***, is introduced to overcome the outlined limitations. The
idea is to have Access List with a *spec.type* set to "static". Creating the Access List with the
new static type will disable reviews and therefore make it possible to manage such Access Lists
using IaC tools.

## Details

### Glossary

- ***static Access List*** - Access List with the *.spec.type* field set to "static".

### UX

#### Terraform

There will be a new Terraform resource named `teleport_access_list_member`.

```hcl
resource "teleport_access_list" "crane_operation" {
  header = {
    version = "v1"
    metadata = {
      name = "crane-operation"
    }
  }
  spec = {
    # type must be set to "static" to manage members with Terraform.
    type = "static"
    title = "Crane operation"
    description = "Used to grant access to the crane."
    owners = [
      {
        name        = "gru"
        description = "The supervillain."
      }
    ]
    grants = {
      roles = ["crane-operator"]
    }
    # membership_requires is optional.
    membership_requires = {
      roles = ["crane-operation-license"]
    }
  }
}

resource "teleport_access_list_member" "crane_operator" {
  header = {
    version = "v1"
    metadata = {
      name = "crane-operator"
    }
  }
  spec = {
    access_list = teleport_access_list.crane_operation.id
    # membership_kind is 1 for "MEMBERSHIP_KIND_USER" or 2 for "MEMBERSHIP_KIND_LIST"
    membership_kind = 1
    # expires is optional. The member will stay in the list after it expires but will lose the
    # grants. expires can be updated.
    expires = "2025-07-28T22:00:00Z"
  }
}
```

It is also possible to add a nested Access List member. The Access Lists members can be created
manually or with an integration (i.e. it also works for Access Lists created with [Okta
integration](https://goteleport.com/docs/identity-governance/okta/app-and-group-sync/) and
[Microsoft Entra ID
integration](https://goteleport.com/docs/identity-security/integrations/entra-id/#how-it-works)).

For example to add an Access List member to the *crane-operation* Access List defined above:

```hcl
resource "teleport_access_list" "nested" {
  header = {
    version = "v1"
    metadata = {
      name = "nested"
    }
  }
  spec = {
    title = "Nested Access List example"
    description = "Used to present how to create a nested Access List member."
    owners = [
       { name = "nested_list_owner" },
    ]
    grants = {
      roles = ["nester"]
    }
  }
}

resource "teleport_access_list_member" "crane_operation_nested" {
  header = {
    version = "v1"
    metadata = {
      name = teleport_access_list.nested.id
    }
  }
  spec = {
    access_list = teleport_access_list.crane_operation.id # defined in the example above
    membership_kind = 2 # 1 for "MEMBERSHIP_KIND_USER", 2 for "MEMBERSHIP_KIND_LIST"
  }
}
```

To add a member which is an Access List created for an Okta group:

```hcl
resource "teleport_access_list_member" "crane_operation_okta_group" {
  header = {
    version = "v1"
    metadata = {
      name = "00gt3c8z9ukePm5uF697" # taken from Access List URL: https://my-company.teleport.sh/web/accesslists/00gt3c8z9ukePm5uF697
    }
  }
  spec = {
    access_list = teleport_access_list.crane_operation.id # defined in the example above
    membership_kind = 2 # 1 for "MEMBERSHIP_KIND_USER", 2 for "MEMBERSHIP_KIND_LIST"
  }
}
```

There are a few things to note here:

- *access_list.spec.owners* - as static Access Lists don't support reviews, owners don't serve any
  special purpose other than RBAC - owners are allowed to add members to the list, but this can
  happen only through Terraform or tctl right now
- fields not present in *teleport_access_list* resource:
  - *.spec.audit* - reviews are disabled for static Access Lists and specifying it results in an error
  - *.spec.ownership_requires* - allowed but skipped because *owners* are skipped
- fields not present in *teleport_access_list_member* resource:
  - *.spec.name* - when not set, defaults to *.header.metadata.name*
- unfortunately we have to user integers for *membership_kind* because of how `protoc` generates
  schema code from proto files

The new resource only allows managing members for the *access_list* with *.spec.type* set to
"static". If the *type* is not set to "static":

```
teleport_access_list_member.crane_operator: Creating...
╷
│ Error: Error reading Member
│
│   with teleport_access_list_member.crane_operator,
│   on main.tf line 61, in resource "teleport_access_list_member" "crane_operator":
│   61: resource "teleport_access_list_member" "crane_operator" {
│
│ member's access_list is not static type
╵
```

The *access_list* type cannot be modified once it's created:

```
teleport_access_list.crane_operation: Modifying... [id=crane-operation]
╷
│ Error: Error updating AccessList
│
│   with teleport_access_list.crane_operation,
│   on main.tf line 15, in resource "teleport_access_list" "crane_operation":
│   15: resource "teleport_access_list" "crane_operation" {
│
│ access_list "crane-operation" type "static" cannot be changed to ""
╵
```

The *.spec.audit* field is illegal for static *access_list*:

```
teleport_access_list.characters: Creating...
╷
│ Error: Error creating AccessList
│
│   with teleport_access_list.characters,
│   on main.tf line 14, in resource "teleport_access_list" "characters":
│   14: resource "teleport_access_list" "characters" {
│
│ audit not supported for non-reviewable access_list of type "static"
╵
```

Trying to create a member of non-static access_list:

```
teleport_access_list_member.fighter: Creating...
╷
│ Error: Error reading Member
│
│   with teleport_access_list_member.fighter,
│   on main.tf line 42, in resource "teleport_access_list_member" "fighter":
│   42: resource "teleport_access_list_member" "fighter" {
│
│ rpc error: code = Unknown desc = member.spec.access_list must reference an access_list of static
│ type (i.e. with spec.type set to "static"). Member "fighter" cannot be added to access list
│ "characters" because access list "characters" is not of "static" type. Teleport IaC tools support
│ adding members only to "static" access lists.
╵
```

Trying to import non-static access_list member:

```
teleport_access_list_member.fighter: Importing from ID "characters/fighter"...
╷
│ Error: Error reading Member
│
│ rpc error: code = Unknown desc = member.spec.access_list must reference an access_list of static
│ type (i.e. with spec.type set to "static"). Member "fighter" cannot be added to access list
│ "characters" because access list "characters" is not of "static" type. Teleport IaC tools support
│ adding members only to "static" access lists.
╵
```

Member's *spec.name* is not empty and not equal to *metadata.name*:

```
teleport_access_list_member.fighter: Creating...
╷
│ Error: Error creating Member
│
│   with teleport_access_list_member.fighter,
│   on main.tf line 36, in resource "teleport_access_list_member" "fighter":
│   36: resource "teleport_access_list_member" "fighter" {
│
│ request's member.header.metadata.name "fighter" and member.spec.name "wizard" must be equal
│ (alternatively spec.name can be empty). Tip: There can be multiple members with the same
│ metadata.name if they have different spec.access_list set (i.e. they belong to different access
│ lists).
╵
```

#### Other tools

- `tctl` - can modify members of the *static* *access_list* resources with the existing `acl users`
  commands and `create -f` command.
- web UI - for the first iteration it won't be possible to modify members of the *static* *access_list*
  resources, but we are open to implement that if the need arises. This will however require a bit more work and
  thought of how to create/modify *static* *access_list* resources themselves in the web UI. As the
  first step all the fields in the Access List view will be grayed out with the proper information
  displayed (be it a tooltip pop-up or a message somewhere on the screen).
- `teleport-operator` - won't have support to reduce the scope but the possibility is open too.

### Proto specification

We want Terraform *teleport_access_list_member* resources to be created only for the *static*
Access Lists. To achieve that on the server side, a new set of gRPCs for static members management
will be exposed.

- GetStaticAccessListMember
- UpsertStaticAccessListMember
- DeleteStaticAccessListMember

All the new gRPCs have _Static_ in the name and will only allow member management for the
access_list resources of "static" type.

The API should be similar to the existing *non-Static* endpoints. E.g. for
`UpsertStaticAccessListMember`:

```protobuf
service AccessListService {
  ...
  // UpsertStaticAccessListMember creates or updates an access_list_member resource. It returns
  // error and does nothing if the target access_list is not of type static. This API is there for
  // the IaC tools to prevent them from making changes to members of dynamic access lists.
  rpc UpsertStaticAccessListMember(UpsertStaticAccessListMemberRequest) returns (UpsertStaticAccessListMemberResponse);
  ...
}

// UpsertStaticAccessListMemberRequest is the request for upserting an access_list_member. It fails
// if the access_list is not static type.
message UpsertStaticAccessListMemberRequest {
  // member is the access_list_member to upsert.
  Member member = 1;
}

message UpsertStaticAccessListMemberResponse {
  // member is the upserted access_list_member.
  Member member = 1;
}
```

### Security model

There is no restriction on how static Access Lists members can modified on the RBAC level. All the
"obstacles" to modify the static Access Lists and their members are the UI tweaks only, and their
purpose is the user's guidance on how to properly utilize static Access Lists.

In other words the existing gRPCs (e.g. `UpsertAccessListWithMembers`) can be still used to modify
static Access Lists and their members but:

- we don't allow static Access Lists creation/modifications in the web UI and it's blocked on the
  proxy level
- Terraform provider can create/modify any Access List, but is can only modify members of the
  static Access Lists (enforced by using `*Static*` gRPCs)
- `tctl` can modify any Access List (with `create -f` and `edit access_list/<name`) and its members
  (with `acl users add/rm`) freely
- Access List and its members validation is different depending on the type

### Backward compatibility

If a cluster with Access Lists of the new "static" type is created and the downgraded to a version
not supporting the new type:

- if the static Access List is only being read, nothing happens
- if the static Access List is modified in the UI in any way, including modifying its members it
  ill be converted to a regular Access List, with audit set

To better illustrate this, let's consider this scenario:

- a static Access List is created with Terraform
- cluster is downgraded
- the static list is modified in the downgraded cluster version
- the cluster is upgraded back again

When Terraform is being run again, there will be errors like this:

```
╷
│ Error: Error updating AccessList
│
│   with teleport_access_list.characters,
│   on main.tf line 14, in resource "teleport_access_list" "characters":
│   14: resource "teleport_access_list" "characters" {
│
│ audit not supported for non-reviewable access_list of type "static"
╵
```

```
╷
│ Error: Error reading Member
│
│   with teleport_access_list_member.npcs,
│   on main.tf line 76, in resource "teleport_access_list_member" "npcs":
│   76: resource "teleport_access_list_member" "npcs" {
│
│ rpc error: code = Unknown desc = member.spec.access_list must reference an access_list of static type (i.e. with spec.type set to "static"). Member "npcs" cannot be added to access list
│ "characters" because access list "characters" is not of "static" type. Teleport IaC tools support adding members only to "static" access lists.
╵
```

**NOTE:** This can be very confusing if the HA cluster runs two versions (one with the static Access
*Lists support, and one without) and the feature is being used.

It can mean, that in case of downgrade and upgrade everything has to be potentially removed
(including the resources in the Terraform state) and started over again.

### Audit events

Audit events will be exactly the same as the current Access List membership related events.

### Test plan

*Access Lists* section of the test plan should be extended with points verifying that:

- it is not possible to set *audit*
- Access List type cannot be changed
- appropriate web UI elements are disabled for static Access Lists
- Access List members can be managed with Terraform only for Access List of type static
- Member *.spec.name* defaults to the resource name
- When *.spec.name != .metadata.name* a clean error is displayed
