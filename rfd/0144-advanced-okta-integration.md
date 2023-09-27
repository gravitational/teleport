---
authors: Trent Clarke (trent@goteleport.com)
state: draft
---

# RFD 00144 - Advanced Okta Integration

## Required approvers

* Engineering: @r0mant && ??
* Product: @klizhentas || @xinding33
* Security: @reedloden || @jentfoo

## What

This RFD describes the expansion of Teleport's integration with
Okta.

The overall goals of this integration are to:

 1. Simplify (i.e. largely automate) the process of configuring
    Teleport to use Okta as an Identity Provider
 3. Show all Okta users in the Teleport UI, including their assigned
    roles/access lists. 
 4. Ensure that changes to the upstream Okta user are immediately 
    reflected in the corresponding downstream Teleport user (as 
    close to _immediately_ as is reasonably practical)
 5. Customize the access Teleport grants those users based on the
    Okta user's profile and/or Okta group membership.

## Why

Teleport currently supports 2 methods of integrating with Okta:

 1. The SAML connector, for SSO login, and
 2. The Okta Sync Service integration, which imports Okta apps and groups
    into Teleport, and keeps them synchronized with the upstream Okta
    system.

### Why is this not sufficient?

 - There is an implicit requirement that the Okta Sync Service points
   to the an upstream Okta Idp already connected to Teleport via a SAML 
   SSO connector, but can easily be configured otherwise.
 - Setting up the SSO connector is tedious and error-prone.
 - There is customer concern that Teleport users are not immediately 
   deleted when the corresponding upstream Okta user is disabled or
   deleted.
 - The existing integrations do not mesh well with the new Access List features
## Details

There are several inter-related features discussed in this RFD

 * **Automated SSO Setup** - reduces the burden on users when setting
   up an SSO connector and ensuring that it is configured to correctly
   interact with the Okta integration service,
 * **User Synchronization** - ensures that Teleport users sourced from
   Okta correctly reflect the state of their upstream Okta counterparts
 * **Implicit Access Lists** - allows traits and group memberships derived
   from Okta to grant access to resources in the Teleport cluster.

## Automated SSO Setup

When the Okta integration service starts it will enumerate the active
applications on the target Okta system. If any existing Application
is found with its SSO URL value set to the appropriate URL for the
Teleport cluster (e.g. `https://example.com:3000/v1/webapi/saml/acs/okta`), then this application will be used as Teleport's SSO gateway into Okta.

> **NOTE FOR REVIEWERS:** I am open to better ways of identifying
> the correct Okta App. Another option would be having the correct 
> `audience` value set.

If no such Okta Application is found, Teleport will automatically
create named "Teleport `${TELEPORT_CLUSTER_NAME}`" and add the 
Okta `Everyone` group to it, granting all Okta users access to
teleport.

Similarly, the integration service will check for a SAML SSO for
the upstream Okta app connector, and if none is found one will
be created.

> **NOTE FOR REVIEWERS:** Any thoughts on what would would identify 
> *"the upstream Okta app connector"* for this purpose
>  1. Mere existence of an SSO Connector named `okta`
>  2. Identical entity descriptor
>  3. Some metadata label
>
>  My naïve guess would be (2), but any contributions are appreciated.

The resulting SAML SSO connector will be named `okta` as per our 
existing Okta integrator guide. It will be created with an unsatisfiable 
attributes-to-roles mapping, so that any user logging in through the
SSO connector _not already been created via the sync service_ will
not be granted any  access in the Teleport cluster when the attempt
to log in.

Any existing SSO connector named `okta` will be overwritten.

### Logging in via SSO

When a user logs in via SAML, the SSO connector currently creates 
an ephemeral Teleport user that expires after some timeout. For any
subsequent logins that occur during that time, Teleport simply
overwrites the user record with a new timeout.

Where this becomes a problem is when a long-lived, sync-service
managed user logs in; the connector will overwrite the user
record, destroying any traits set by the Okta sync service and
setting an expiry date on that user as well.

If the SAML SSO connector detects that the user that has logged in
has a sync-service-created, long-lived user then the connecter _will
not_ overwrite it.

> NOTE FOR REVIEWERS: The PoC determination is made by checking 
> for the absence of an expiry time, but in production it will
> be probably be done via some attribute set by the sync service.
> The trick will be picking some attribute that entangle 

### Deleting the integration

Deleting the Okta integration does **not** cascade to any SSO connector
that it created. 

## User Synchronization

Because Okta imposes strict rate limits on API calls, and their API
is not particularly friendly to bulk operations, we will take a 
two-pronged approach to synchronizing the Okta users. 

 1. Periodically polling Okta user lists and reconciling them against
    the current state of Teleport.
 2. Receiving real-time update events from the Okta event webhooks,
    in order to reflect real-time changes as quickly as possible.

Note: See the _Staged Delivery_ section for details on how the sync service
is expected to evolve as the various stages are delivered.

### Polling & reconciliation

The Sync Service will periodically poll the upstream Okta service for
all active Okta users and groups, building a matrix of users and the
groups they belong to, and use that information to update the Teleport 
users database. 

This is a relatively API-intensive operation (especially for an Okta
organisation with many groups) and as such must happen relatively infrequently
so as to avoid Okta throttling our API calls. The actual interval to be used 
will be defined by experiment, but is expected to be in the order of hours
rather than minutes.

### Reconciliation Algorithm

#### Step 1: Generate a list of candidate Teleport users from the upstream Okta user database:
1. Fetch all Okta Users
2. Fetch all Okta Groups
3. For each group, fetch the member list of that group
4. Build a matrix that maps users to the groups they are members of 
5. Create candidate Teleport user records for each user, including 
   translating all Okta user profile data as Teleport user traits and
   including group memberships as a synthetic trait 

> Note: We _can_ save a few Okta API calls by constructing the user list 
> from the group members lists and dropping duplicate users, as we get a
> full user record for each group member. It's in the above algorithm
> separately for clarity, but the actual method we end up using is
> considered an implementation detail.

#### Step 2: List all downstream Teleport users marked as being created by the sync service

A Teleport user is included in this list if they have a

1. `teleport.dev/origin` label with the value `okta`, *and*
2. `teleport.internal/okta-user-id` label with any value
   
We will refer to this as the *Extant User List*, but it may help to think of it
in garbage collection terms as the *condemned set*. To survive reconciliation,
users in this list must be shown to to have a corresponding upstream Okta user, 
as defined by the candidate user list.

#### Step 3: Reconcile the *Candidate* and *Extant* user lists

1. For each candidate user
   1. If there is no Teleport user with the same `okta-user-id` label in the Extant User List:
      * Create the Teleport user. 
      * If the Teleport login (i.e. email address) is taken the service 
        logs a warning and skips the user
   2. If a Teleport user with the same `okta-user-id` label *does* exist in the Extant User List:
      * If the extant User has a different username to the candidate user, the
        extant user is deleted from Teleport and  re-create the Teleport user
        using the candidate data.
      * If the extant user is different to the candidate user, the 
        extant Teleport user is updated to match the candidate. A user is 
        considered *different* if any labels, traits or roles do not
        match.
      * Remove the extant user from the Extant Teleport Users List.
2. All Teleport users remaining in the Extant User List at this 
   point have not been referenced by a candidate user during reconciliation,
   and therefor represent users deleted or deactivated in the upstream Okta
   user database. Any users remaining in the extant user list are deleted
   from Teleport.

#### Sync-Service Created Users

The "candidate" users created in Step 1, and injected into the Teleport user
database by Step 3, are generated by taking the Upstream Okta user record, and
creating a Teleport user record with:

 * New `okta` user type (as opposed to `local` or `saml` )
 * Okta user profile values converted to traits
 * Group membership expressed as traits 
 * All okta-supplied traits come prefixed with `okta/`

For example:

```yaml
version: v2
metadata:
  name: hiro@example.com
  labels:
    okta/org: https://example.okta.com
    teleport.dev/origin: okta
    teleport.internal/okta-user-id: 00ub1q9yfsRSfO91a5d7
spec:
  created_by:
    connector:
      id: Okta Service
      identity: 00ub1q9yfsRSfO91a5d7
      type: okta
    time: '2023-09-20T22:58:10.840383+10:00'
    user:
      name: system
  expires: '0001-01-01T00:00:00Z'
  roles:
    - editor
    - access
  traits:
    okta/email:
      - hiro@example.com
    okta/firstName:
      - Hiro
    okta/lastName:
      - Protagonist
    okta/group-ids:
      - 00gb0c5lmzAl5GbZc5d7
      - 00gb3j5tpspkQdnWa5d7
    okta/groups:
      - Everyone
      - Deliverators
    okta/login:
      - hiro@example.com
```

### Real-time updates (a.k.a *Dead Reckoning*)

The sync service will expose a webhook endpoint that will receive real-time
notifications from Okta. 

Note that User and group records received during a poll-and-reconcile 
operation will be considered authoritative, and no attempt to preserve
changes introduced by the real-time event hooks.

#### Webhook Authentication

Okta webhooks will authenticate using a shared secret. The secret will be 
generated when the plugin is installed, and stored in a `plugin_static_credentials`
resource.

This shared secret will be supplied by Okta when invoking a webhook. 

#### Why not SCIM?

  - Cannot be provisioned via REST API
  - Doesn't give us anything over Webhooks for Okta support 
  - Even if we'd prefer to use an open standard like SCIM, the
    SCIM User and Groups schemas are still pretty vaguely defined,
    suggesting that we would still require special handling for
    importing users from different IdP vendors.

### Deleting the integration

Deleting the Okta integration does **not** cascade to the users that
it created. 

## Implicit Access Lists

The current implementation of Access Lists requires that a user meet two 
requirements before being considered a member of a list: 
 * they must be added to the list by an administrator, and 
 * they must meet the `membership_requires` conditions at the time that
   list membership is evaluated (for example, at an access check).

The idea of an _implicit_ access list extends this by removing the first
condition. In an Implicit Access List, any user meeting the `membership_requires`
conditions at evaluation time is implicitly considered a member of the list.

While this is a useful property in its own right, it intersects with 
the Okta sync service as it removes the need for any extra machinery to
synchronize Access Lists to match the changing users coming from the
upstream Okta system.

Due to the Access List requirement that a User must meet the `membership_requires`
conditions at the time membership is evaluated, most of the machinery 
required to implement Implicit Access Lists is already in place.

### Resource Changes

To mark an Access List as *implicit*, we will add a `membership_evaluation` 
field to the resource spec, with two possible values: 
 * `explicit` - A user must be an explicitly-included member of the list to be 
   considered a member. This is the default behaviour for backwards 
   compatibility with existing lists.
 * `implicit` - A user need only meet the `membership_requires` conditions at 
   evaluation time to be considered a member of the list

Example:

```yaml
version: v1
kind: access_list
metadata:
  name: ea6cccbe-ceac-4776-8a89-4b1365fc03f5
spec:
  title: "Access List Title"
  description: "A description of the Access List and its purpose"
  membership_evaluation: "implicit"
  # rest of the resource as per existing resource
```

### UX Changes

The backends used by `tctl acl ls` and the Web UI will have to be updated to
reflect that an Access List is Implicit, and to generate the list content at
query time. 
## Backwards Compatibility

The activation of user sync and automatic SSO connector creation features will
be controlled by new flags in the Okta plugin resource spec, with the features
defaulting to *disabled*. This is done in order to prevent surprising behaviour
in existing integrations when the new features are introduced in Teleport.

This is not intended to be a generally available set of feature flags, and the 
user will have no choice in the the enabled feature set. Any newly-created
plugin resource will automatically have *all* of the Okta features supported
by the creating version of Teleport enabled.
## Staged release

This is suggested structuring of the work required, such that each step delivers 
useful features with minimal re-work in later stages.
### Stage 1: Automation & Basic sync
 * Automated creation of SSO SAML connector
 * Limited Okta user sync
   * Polling Only
   * Teleport user created/destroyed in response to changes in Okta 
   * **No** group memberships pulled from Okta (drastically reduces Okta API hit count)
   * All users automatically get `access` and `requester` roles on creation. 
 * Web UI treats okta users like local users (for example, Okta users may have traits 
   added/removed, user may be deleted)

### Stage 2: Implicit access lists

I've included this before other, Okta-specific work because:
 * it is useful in its own right, 
 * it enables the following Okta-specific work to be immediately useful, rather 
   than having to wait for this to be implemented afterwards. 

### Stage 3: Real-time updates
 * Webhook handling
 * Okta Group membership and traits
  
### Stage 4: Enhanced UI & Automatic access list creation
 * User presented with traits list during plugin installation
 * Automatic creation of Implicit Access Lists based on selected traits.
