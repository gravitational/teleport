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

- ***static Access List*** - Access List with a "static" *sub_kind*.

### UX

#### Terraform

There will be a new Terraform resource named `teleport_access_list_member`. 

```hcl
resource "teleport_access_list" "crane_operation" {
  header = {
    version = "v1"
    metadata = {
      sub_kind = "static"
      name = "crane-operation"
    }
  }
  spec = {
    title = "Crane operation"
    description = "Used to grant access to the crane."
    grants = {
      roles = ["crane-operator"]
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
    name = "crane-operator" 
    access_list = teleport_access_list.crane_operation.id
    membership_kind = 1 // 1 for "MEMBERSHIP_KIND_USER", 2 for "MEMBERSHIP_KIND_LIST"
  }
}
```

There are a few things to note here:

- fields not present and not allowed in *teleport_access_list* resource:
  - *owners* - owners are optional with static Access Lists as they don't serve any major purpose
  - *membership_requires*/*ownership_requires* - eligibility criteria is disabled for static Access
    Lists
  - *audit* - reviews are disabled for static Access Lists
- unfortunately we have to user integers for *membership_kind* because of how `protoc` generates
  schema code from proto files

The new resource only allows managing members for the "static" *access_list* *sub_kind*. If the *sub_kind* is not set to "static":

```
teleport_access_list_member.crane_operator: Creating...
╷
│ Error: Error creating Member
│ 
│   with teleport_access_list_member.crane_operator,
│   on main.tf line 61, in resource "teleport_access_list_member" "crane_operator":
│   61: resource "teleport_access_list_member" "crane_operator" {
│ 
│ member's access_list is not static subkind
╵
```

#### Other tools

- `tctl` - can modify members of the *static* *access_list* resources with the existing `acl users`
  commands and `create -f` command.
- web UI - for the first iteration it won't be able to modify members of the *static* *access_list*
  resources, but we are open to implement that. This will however require a bit more work and
  thought of how to create/modify *static* *access_list* resources themselves in the web UI.
- `teleport-operator` - won't have support to reduce the scope but the the possibility is open too.

### Proto specification

We want Terraform *teleport_access_list_member* resources to be created only for the *static*
Access Lists. To achieve that on the server side, a new set of gRPCs for static members management
will be exposed.

Current list of gRPCs for access_list_member:

- CountAccessListMembers
- ListAccessListMembers
- ListAllAccessListMembers
- GetAccessListMember
- GetAccessListOwners
- UpsertAccessListMember
- UpdateAccessListMember
- DeleteAccessListMember
- DeleteAllAccessListMembersForAccessList
- DeleteAllAccessListMembers

Two new gRPCs will be added:

- GetStaticAccessListMember
- UpsertStaticAccessListMember
- DeleteStaticAccessListMember

All the new gRPCs have _Static_ in the name and will only allow member management for the
access_list resources with "static" sub_kind.

The API should be exactly the same as for the existing *non-Static* endpoints. E.g. for
`UpsertStaticAccessListMember`:

```protobuf
service AccessListService {
  ...
  // UpsertStaticAccessListMember creates or updates an access_list_member resource. It fails if
  // the target access_list is not static (i.e. does't have static_access_list subkind).
  rpc UpsertStaticAccessListMember(UpsertStaticAccessListMemberRequest) returns (Member);
  ...
}

// UpsertStaticAccessListMemberRequest is the request for upserting an access_list_member. It fails
// if the access list is not static (i.e. does't have static_access_list subkind)
message UpsertStaticAccessListMemberRequest {
  reserved 1, 2, 3;
  reserved "access_list", "name", "reason";

  // member is the access list member to upsert.
  Member member = 4;
}
```

### Backward compatibility

**Breaking:** If a *static Access List* is created with owners field empty, then it is impossible
*to downgrade the cluster to the previous version without breaking the cache. This can be recovered
*only by deleting all static Access Lists **before the downgrade**. This is due to validation code
*in the cache which checks if Access Lists have non-empty owners set. The alternative is to set the
*owners, but it also has to happen before the downgrade.

This breaking change will be outlined in the changelog and Terraform *teleport_access_list_member*
resource documentation.

### Audit events

Audit events will be exactly the same as the current Access List membership related events.

### Test plan

*Access Lists* section of the test plan should be extended with points verifying that:

- it is not possible to set, *owners*, *membership_requires*, *ownership_requires* and *audit*
- Access List members can managed with Terraform only for "static" Access List *sub_kind*
