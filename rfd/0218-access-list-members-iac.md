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

Currently the Access List membership model is very dynamic in nature. Periodic membership reviews
are required, membership can expire, and custom dynamic eligibility criteria can be specified.
Because of that, the IaC approach to Access List membership was not provided so far and we don't
have a good way to introduce it for the Access Lists in their current form.

Manual management of Access List membership doesn't always scale. There are ways of proper
structuring teams as Access Lists and them using the nested Access List concept to assign teams to
resources, but that doesn't work when users are managed externally.

The concept of dynamically assigning users to Access Lists (something like membership criteria)
also won't scale when users are managed externally in large organizations. For example Microsoft
Entra ID won't display any groups in SAML assertion [if the user is assigned to more than 150
groups](https://learn.microsoft.com/en-us/entra/identity/hybrid/connect/how-to-connect-fed-group-claims).

A new concept, a ***static Access Lists***, is introduced to overcome the outlined limitations. The
idea is to have Access List with a *sub_kind* set to "static". Creating the Access List with the
new *sub_kind* will disable the dynamic features of the *static* Access List (like reviews,
expiration, or eligibility criteria). This will make it possible to manage such Access Lists using
IaC tools. 

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
    // membership_kind is 1 for "MEMBERSHIP_KIND_USER" or 2 for "MEMBERSHIP_KIND_LIST"
    membership_kind = 1
    // expires is optional. The member will stay in the list after it expires but will lose the
    // grants. expires can be updated.
    expires = "2025-07-28T22:00:00Z"
  }
}
```

There are a few things to note here:

- fields not present in *teleport_access_list* resource:
  - *.spec.audit* - reviews are disabled for static Access Lists and specifying it results in an error
  - *.spec.owners* - owners are optional with static Access Lists as they don't serve any major purpose
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
│ access_list "crane-operation" type "static" cannot be changed to "dynamic"
╵
```

The *.spec.audit* field is illegal for static *access_list*:

```
teleport_access_list.crane_operation: Modifying... [id=crane-operation]
╷
│ Error: Error reading AccessList
│
│   with teleport_access_list.crane_operation,
│   on main.tf line 15, in resource "teleport_access_list" "crane_operation":
│   15: resource "teleport_access_list" "crane_operation" {
│
│ Can not convert *accesslist.AccessList to AccessList: audit not supported for static access_list
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
  // UpsertStaticAccessListMember creates or updates an access_list_member resource. It fails if
  // the target access_list is not static (i.e. does't have "static" subkind).
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

**Breaking:** If a *static Access List* is created with owners field empty, then it is impossible
to downgrade the cluster to the previous version without breaking the cache. This can be recovered
only by deleting all static Access Lists **before the downgrade**. This is due to validation code
in the cache which checks if Access Lists have non-empty owners set. The alternative is to set the
owners, but it also has to happen before the downgrade.

This breaking change will be outlined in the changelog and Terraform *teleport_access_list_member*
resource documentation.

### Audit events

Audit events will be exactly the same as the current Access List membership related events.

### Test plan

*Access Lists* section of the test plan should be extended with points verifying that:

- it is not possible to set *audit* and *owners* are optional
- Access List type cannot be changed
- appropriate web UI elements are disabled for static Access Lists
- Access List members can be managed with Terraform only for Access List of type static
- Member *.spec.name* defaults to the resource name
