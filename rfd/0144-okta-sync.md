---
authors: Trent Clarke (trent@goteleport.com)
state: draft
---

# RFD 00144 - Okta integration

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

## Details

### Automated SSO Setup

When the integration service starts it will enumerate the active
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

### Sync-Service Created Users
 * New `okta` user type
 * Okta profile converted to traits
 * Group membership expressed as traits
 * All okta-supplied traits come prefixed with `okta/`

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
      - okta-dev
    okta/login:
      - hiro@example.com
```

 - mark with source
 - separate user type (not local, not saml)
 - preserve roles where added by admins
   - this becomes impossible when we have a custom attribute to role
     mapping, as we have no way of knowing _how_ a role was assigned
     to a user.
   - May not be an issue, as we may will be dealing in access lists
     by then, anyway

### Logging in

When a user logs in via SAML, the SSO connector currently creates 
an ephemeral Teleport user that expire after some timeout. For any
subsequent logins that occur during that time, Teleport simply
overwrites the user record with a new timeout.

Where this becomes a problem is when a long-lived, sync-service
managed user logs in - the connector will overwrite the user
record, destroying any traits set by the Okta sync service and
setting an expiry date on that yser as well.

If the SAML SSO connector detects that the user that has logged in
has a sync-service-created, long-lived user then the connecter _will
not_ overwrite it.

> NOTE FOR REVIEWERS: The PoC determination is made by checking 
> for the absence of an expiry time, but in production it will
> be probably be done via  

### Synchronization

Because Okta imposes strict rate limits on API calls, and their API
is not particularly friendly to bulk operations, we will take a 
two-pronged approach to synchronizing the Okta users. 

 1. Periodically polling Okta user lists and reconciling them against
    the current state of Teleport. Polling will be infrequent (intervals 
    of at least 10 minutes, perhaps closer to an hour in  areal deployment).
 2. Receiving real-time update events from the Okta event webhooks,
    in order to reflect real-time changes as quickly as possible.

User and group records received during a poll-and-reconcile operation
will be considered authoritative, and no attempt to preserve changes 
introduced by the real-time event hooks.

Note: During the initial delivery, Teleport _will_ preserve roles added
      to the Okta users 

#### Polling

#### Real-time updates (a.k.a *Dead Reckoning*)

#### Changed Okta login

It is possible for an Okta user to changer their login name without
deleting and re-creating their account. This is not possible with
Teleport, so an Okta user changing their login will be treated as a
user deletion & re-creation by Teleport.

Teleport users are, beyond what is specified in their records, 
stateless and so re-creating a user like this should be safe. 

Both user records will be tagged with the upstream Okta ID so that
both Teleport users are traceable to each other, and back to the 
upstream Okta system. 

#### Deleting the integration

### Synchronization

### Access Control

### Dynamic Access Lists

Teleport 

### Dead reckoning and reconciliation

### Dead Reckoning

Webhooks

#### Why not SCIM?

  - Cannot be provisioned via REST API
  - Doesn't give us anything over Webhooks, which can be
  - Even if we'd prefer to use an open standard like SCIM, the
    we User and Groups schemas are still pretty vaguely defined,
    suggesting that we would still require special handling for
    importing users from different IdP vendors.

### Staged release

#### Stage 1: Automation

 - Automate creation of SAML connector
 - Users automatically get `access` and `requester` roles
 - Synchronise users with Okta 