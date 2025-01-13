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

 1. Simplify (i.e. largely automate) the process of configuring Teleport to use
    Okta as an Identity Provider
 2. Show all Okta users in the Teleport UI, including their assigned roles and
    access lists.
 3. Ensure that changes to the upstream Okta user are immediately reflected in
    the corresponding downstream Teleport user (as close to _immediately_ as is
    reasonably practical)
 4. Customize the access Teleport grants those users based on the Okta user's
    profile and/or Okta group membership.

## Why

Teleport currently supports 2 methods of integrating with Okta:

 1. The SAML connector, for SSO login, and
 2. The Okta Sync Service integration, which imports Okta apps and groups into
    Teleport, and keeps them synchronized with the upstream Okta system.

### Why is this not sufficient?

 - There is an implicit requirement that the Okta Sync Service points to the an
   upstream Okta Idp already connected to Teleport via a SAML
   SSO connector, but can easily be configured otherwise.
 - Setting up the SSO connector is tedious and error-prone.
 - There is customer concern that Teleport users are not immediately deleted
   when the corresponding upstream Okta user is disabled or deleted.
 - The existing integrations do not mesh well with the new Access List features.

## Details

There are several inter-related features discussed in this RFD

 * **Automated SSO Setup** - reduces the burden on users when setting up an SSO
   connector and ensuring that it is configured to correctly interact with the
   Okta integration service,
 * **User Synchronization** - ensures that Teleport users sourced from Okta
   correctly reflect the state of their upstream Okta counterparts
 * **Implicit Access Lists** - allows traits and group memberships derived from
   Okta to grant access to resources in the Teleport cluster.

## Running the Okta service

The Okta service is expected to be started in two different ways:

 * As Teleport service (in the same way that protocol agents currently, for
   example), and configured via the Teleport config file. This is assumed to be
   the primary method used by self-hosted Teleport.
 * As a hosted integration, configured and managed via the creation and/or
   deletion of a `plugin` resource.

By default, anything described in this RFD refers to both run modes. The
RFD will indicate where features or tooling refer specifically to one
mode or the other.

It is not anticipated that the Okta service itself will need to behave
any differently in either scenario.

## Configuration

Both startup modes will use the same configuration schema, the only
difference is how that schema is sourced:

 * **Self-Hosted Service**: The Okta Sync Service is configured from a
   stanza in the Teleport config file.
 * **Hosted integration**s: Any per-cluster configuration will be stored in the
  `plugin` resource, which will be combined with appropriate default values to 
  generate a configuration block at runtime, which is then given to the Okta 
  Sync Service to configure itself.

### Authenticating with Okta

The Advanced Okta Integration is largely implemented as an extension of the
existing [Okta
service](https://goteleport.com/docs/enroll-resources/application-access/okta/),
and uses the same API Key approach to Authentication.

For self-hosted services, the Okta API key is supplied via the Teleport 
config file.

For hosted integrations, this key is stored as a `plugin_static_credentials`
resource as per the existing integration.

### Permissions required

Sourced from the Okta Admin role [permissions list](https://help.okta.com/en-us/content/topics/security/custom-admin-role/about-role-permissions.htm).

| Okta Permission/Role  | Reason                                |
|-----------------------|---------------------------------------|
| Manage Applications   | Creating SAML App for SSO connector   |
| View Groups<br>Edit application's user assignments<br> Edit groups' application assignments | Adding users to SAML App for SSO Connector |
| View Groups<br>View Users and their details | Sync users and groups to Teleport |

### Creating a hosted integration

The Okta sync service is started like any other hosted integration:

 1. The Teleport Web UI handler creates a `plugin` CRD in response
    to an admin hitting the "install" button
 2. The Teleport plugin manager picks up the new `plugin` CRD,
    determined the integration to run based on which spec field
    is populated - in this case `okta` - and starts the appropriate
    service.

In future work the user will be presented with a richer UI for configuring
Access Lists based on traits derived from the upstream Okta users, but the
actual initial launch mechanism is intended to remain largely the same. The
extra UI will have to wait for the integration service to start up and perform
an initial user sync before these values are available.

The rough intention is to expose the values via extensions to the existing
Okta GRPC service.

### Deleting the hosted integration

If the cluster admin deletes the hosted integration that triggers
the Okta Sync Service, the deletion does not **not** cascade to
any User or SSO connector that it created.

## Automated SAML SSO Setup

The Automated SAML Connector Setup will be performed as part of the Okta
hosted plugin enrolment process. No attempt will be made to integrate it
into the self-hosted, agent-style process.

The SAML Connector setup will look for a SAML connector with a configuration-
supplied name.

> NOTE: The default connector name will be `okta-integration`. There will initially
>       be *NO* UI for hosted integrations to override this, but there is no 
>       technical reason precluding this being added later.

If no such connector exists, the automation will proceed to:

 1. Create an Okta SAML Application named "Teleport `${TELEPORT_CLUSTER_NAME}`",
 2. Add the built-in Okta `Everyone` group to the newly-created app, granting
    all Okta users in the system access to log into Teleport via Okta.
 3. Download the new Okta SAML app entity metadata
 4. Create a new Teleport SAML SSO connector, using the configured name and
    the ingested Okta-supplied SAML entity metadata.

It is safe to automatically create multiple Okta applications with the same
name, as the displayed app name is just a label and not the app's primary ID. As
such, we will make no attempt to enforce uniqueness on the Okta Application
names.

The Teleport SAML connector will be created with an attributes-to-roles mapping
that grants all users that log in via SSO the `requester` role, so that these
users will have no access to cluster resources by default but may request access
to them as necessary. The Teleport admin may edit the SSO connector specification
after installation to create a more sophisticated mapping if they so choose.

The SAML connector will be created with labels identifying
 * the SAML connector as "belonging" to an Okta integration
 * the upstream Okta organization it connects to, and
 * the Okta SAML Application that the connector uses to allow Okta login

If such a connector _does_ already exist, the automated SAML connector setup
will verify that the application refers to the same upstream organization using
the SAML connector labels. 

### Logging in via SSO

When a user logs in via SAML, the SSO connector currently creates an ephemeral
Teleport user that expires after some timeout. For any subsequent logins that
occur during that time, Teleport simply overwrites the user record with a new
timeout.

Where this becomes a problem is when a long-lived, sync-service managed user
logs in; the connector will overwrite the user record, destroying any traits set
by the Okta sync service and setting an expiry date on that user as well.

If the SAML SSO connector sees that:

 1. there is an existing teleport user with the login name of the
    user attempting to login, *and*
 2. that user has a `teleport.dev/origin` label with the value `okta`,

... then the SAML connector it will treat the user as a long-lived,
sync-service-created user _will not_ overwrite it.

## User Synchronization

Because Okta imposes strict rate limits on API calls, and their API
is not particularly friendly to bulk operations, we will take a
two-pronged approach to synchronizing the Okta users.

 1. Periodically polling Okta user lists and reconciling them against
    the current state of Teleport.
 2. Receiving real-time update events from the Okta SCIM client,
    in order to reflect real-time changes as quickly as possible.

Note: See the _Staged Delivery_ section for details on how the sync service
is expected to evolve as the various stages are delivered.

### Polling & reconciliation

The Sync Service will periodically poll the upstream Okta service for all active
Okta users and groups, building a matrix of users and the groups they belong to,
and use that information to update the Teleport users database.

This is a relatively API-intensive operation (especially for an Okta
organization with many groups) and as such must happen relatively infrequently
so as to avoid Okta throttling our API calls. The actual interval to be used
will be defined by experiment, but is expected to be in the order of hours
rather than minutes.

We will attempt to avoid triggering the rate limit by explicitly limiting 
ourselves to a certain number of API calls/minute. Such a rate limiter already
exists in the existing integration, and can be reused.

### Reconciliation Algorithm

#### Step 1: Generate a list of candidate Teleport users from the upstream Okta user database:
1. Fetch all Okta Users
2. Fetch all Okta Groups
3. For each group, fetch the member list of that group
4. Build a matrix that maps users to the groups they are members of
5. Create candidate Teleport user records for each user, including translating 
   all Okta user profile data as Teleport user traits and including group
   memberships as a synthetic trait

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
   1. If there is no Teleport user with the same `okta-user-id` label in the 
      *Extant User List*:
      * Create the Teleport user.
      * If the Teleport login (i.e. email address) is taken the service
        logs a warning and skips the user
   3. If a Teleport user with the same `okta-user-id` label *does* exist in the
      *Extant User List*:
      * If the extant User has a different username to the candidate user, the
        extant user is deleted from Teleport and the Sync Service re-creates the
        Teleport user using the candidate data.
      * If the extant user is different to the candidate user, the extant 
        Teleport user is merged with match the candidate. A user is considered
        *different* if any okta-supplied traits do not match.
      * Remove the extant user from the *Extant Users List*.
2. All Teleport users remaining in the *Extant User List* at this point have not 
   been referenced by a candidate user during reconciliation, and therefor
   represent users deleted or deactivated in the upstream Okta user database.
   Any users remaining in the extant user list are deleted from Teleport.

#### User locking

When the sync service detects that an upstream Okta user is deleted, or if a
user is detected transitioning into a lockable state (e.g. `SUSPENDED`), the
Okta sync service will ensure that a Teleport lock is created targeting that
user. 

The purpose of this lock is to terminate any existing session and to prevent a
locked-out user from reusing an already-issued credential to log back into a
Teleport resource. The lock will expire after the maximum Teleport certificate 
TTL, plus some small offset.

As an Okta user can only log into teleport via SAML, Okta itself will prevent
the excluded user from logging back into Teleport to issue new credentials once
the lock has expired.

Example lock resource:
```yaml
kind: lock
version: v2
metadata:
    name: 996d7dbd-0fa0-4618-aa13-8b76202c63b2
    namespace: default
    labels:
        okta/org: https://enzos-pizza.okta.com
        teleport.dev/origin: okta
        teleport.internal/okta-lock-reason: suspended
    expires: 2024-02-08T08:30:48.173519065Z
spec:
    target:
        user: hiro@enzos-pizza.com
    message: Okta user "hiro@enzos-pizza.com" is SUSPENDED
    expires: 2024-02-08T08:30:48.173519065Z
```

Deleted and suspended user locks are differentiated by the `okta-lock-reason`
labels.

When the Teleport Okta sync service detects that an upstream user has
transitioned from a lockable state to an active state (e.g. transitioning from 
`SUSPENDED` to `ACTIVE`) the Okta sync service will delete any suspension
locks (as identified by the `okta/org` & `okta-lock-reason` labels) for that
user. 

> **Note to reviewers:** Is it worth having the sync service check for such
>                        locks and deleting them when a new user is created?

##### Security

The Okta sync service (or any entity with the built-in `Okta` Teleport role) will be
limited to creating and manipulating locks with the `teleport.dev/origin: okta` label set.

#### Sync-Service Created Users

The "candidate" users created in Step 1, and injected into the Teleport user
database by Step 3, are generated by taking the Upstream Okta user record, and
creating a Teleport user record with:

 * New `okta` user type (as opposed to `local` or `saml` )
 * Okta user profile values converted to traits
 * Group membership expressed as traits 
 * All okta-supplied traits come prefixed with `okta/`
 * User is granted the Teleport preset `requester` role 

For example:

```yaml
version: v2
metadata:
  name: hiro@enzos-pizza.com
  labels:
    okta/org: https://enzos-pizza.okta.com
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
    - requester
  traits:
    okta/email:
      - hiro@enzos-pizza.com
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
      - hiro@enzos-pizza.com
```

#### User updates

When an existing user is updated to match an Okta-supplied candidate user:

* the user trait set is merged with the candidate user traits such that
  * all traits prefixed with `okta/` updated with values supplied by the
    candidate user
  * all traits prefixed with `okta/` _not_ in the candidate user's trait
    set are deleted
  * all other traits are preserved
* all role assignments are preserved

This allows the Teleport admin to manually assign roles and traits to an 
Okta-supplied user, while still allowing the upstream Okta admin to control
access to Teleport resources by changing profile values in the Okta 
organization.

### Real-time provisioning via [SCIM](https://en.wikipedia.org/wiki/System_for_Cross-domain_Identity_Management)

The sync service will expose a SCIM implementation that will receive near
real-time notifications from Okta. The exposed webhook endpoint will be served
by the Teleport Proxy as part of the Teleport Web API.

SCIM provisioning for an Okta application _cannot_ be enabled by the Okta API,
and as such we are reliant on the upstream Okta to correctly configure SCIM. This
is a common requirement for any application using SCIM with Okta. We can still
guide the Teleport user through the SCIM setup process as part of the Okta
integration installer.

#### SCIM compatibility

The initial SCIM implementation will target Okta, and as such the initial 
feature set will be limited to what we need to handle requests from the Okta
SCIM client associated with a SAML application.

That said, we expect to handle multiple upstream IdPs in the future and the
design should proceed with that in mind.

#### User Sync

Using SCIM to provision users and sync user data in concert with the Okta Sync 
service presents us with three separate sources of truth for user attributes:
  
  1. The user profile returned from polling the Okta API, as used by the Okta
     Sync Service, which is a flat list of key/value pairs
  2. The structured SCIM user resource, which contains value drawn from (1), but
     mapped to the properties and structures of a SCIM User resource. There is
     no published or well-known mapping between the flat, Okta user-profile key-value 
     pairs and the structure of a SCIM user that I have been able to find. 
  3. The out-of-schema properties that Okta sends alongside the structured SCIM 
     resource, which almost (but not quite entirely) matches the profile from
     (1).

Okta user profiles are also wildly customizable, so the relationships between
these data sources is not something that we can reverse engineer and expect to
work in other Okta deployments.

Reconciling data from these three sources to get a consistent and predictable
synchronization process in an arbitrary Okta deployment is close to impossible,
and as such we're not going to even try. The Okta SCIM implementation will 
ignore the user update in the posted SCIM request, and will use the request as a
trigger event to retrieve the user profile (1) from the Okta API.

This does impose an API call for each user updated, but the impact of that extra
API call should be negligible if updates are infrequent.

#### Fallback 

If the API call overhead from the "triggered polling" update described above
routinely breached Okta's API call rates, or fallback plan is to separate the 
Okta and SCIM attribute sets into separate trait namespaces, and define our own
mappings from the SCIM User schema to Teleport user traits. This would allow
SCIM updates to configure users without requiring an API callback and also
prevent the Okta Sync Service and SCIM server "fighting over" the same values.

As a side effect, it would also allow a much more generic SCIM implementation,
as we are not as dependant on the specifics of the Okta SCIM implementation and
could write to the cross-platform schema.

This would come at the cost of
 * increased storage required for user records
 * more complex merging logic to handle multiple, distinct trait sets

#### Authentication 

Okta supports various authentication schemes for its SCIM client. For the sake
of simplicity, we will use a bearer token, which Teleport will generate at 
enrollment time and supply to the user.

Teleport will store the `bcrypt` hash of this token as a Plugin Static Credential,
with labels to differentiate it from the Okta API key that Teleport uses to
authenticate with Okta. Okta will supply this token in the `HTTP` Authorization
header of its SCIM requests, which Teleport will check against the stored hash.

As Plugin Static Credentials are only readable by `auth`, this requires that at
least some of teh SCIM processing is done at the auth server.

#### Implementation

The SCIM service will require read access to the Plugin Static Credential
database, and as such must run at least partially in a `auth` service. 

Because of the is requirement, the SCIM service will require 

 * A front-end REST API exposed via end points on the Teleport web API running 
   on a Teleport proxy,
 * A back-end running in an Auth service that can read Plugin Static Credential
   database in order to authenticate SCIM requests
 * A GRPC based protocol to communicate between the front and back end

Ideally the implementation will leverage existing scim libraries (e.g. [scim2](https://github.com/scim2/server)
or [elimity-com/scim](https://github.com/elimity-com/scim), but that not work in practice:
At first glance, the existing SCIM libraries seem 
 * strongly coupled to an HTTP implementation (which we can't use directly), and
 * very conservative about what they export, making it harder to implement our
   services in terms of the library types.

Even if using the libraries directly is impractical we can, at the very least,
learn from their design.

### Deleting the integration

Deleting the hosted Okta integration does **not** cascade to SSO connector and 
users that it created.

## Implicit Access Lists

The current implementation of Access Lists requires that a user meet two
requirements before being considered a member of a list:
 * they must be added to the list by an administrator or list owner, and
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

To mark an Access List as *implicit*, we will add a `membership` field to the
resource spec, with two possible values:
 * `explicit` - A user must be an explicitly-included member of the list to be
   considered a member. This is the default behavior for backwards
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
  membership: "implicit"
  # rest of the resource as per existing resource
```

### UX Changes

The backends used by `tctl acl ls` and the Web UI will have to be updated to
reflect that an Access List is Implicit, and to generate the list content at
query time.

## Backwards Compatibility

There are existing Okta hosted integrations in the wild, and we do not want to
unexpectedly change the behavior of those integrations when we release a
Teleport with these new synchronization features.

To this end, all synchronization operations must be disabled by default, and 
only enabled by explicitly setting a value in the plugin resource. 

When the Teleport Plugin Manager finds a `plugin` resource resource with an 
old `PluginOktaSettings`, it will zero-default the unset values and implicitly
disable the new behavior.

## Usage costs

Teleport usage is currently billed in terms of active users per month and 
importing all the users from an upstream Okta account may drastically increase 
the number of users in a Teleport cluster, leading to concerns that using the
Sync Service could cause a client's Teleport bill to increase.

However, "active users per month" is defined as the number of unique usernames
present in login events in a given interval, _not_ the number of users in
Teleport user database - so clients will still be billed in accordance with 
their usage, not the number of defined users.

> **CAVEAT:** Okta users _can change their login names_. Because teleport treats
>             a changed login name as two separate users, if an organization
>             bulk-renames of users (e.g. changing their email address domain),
>             they may be hit with a bill 2x their actual usage.
>
> This is not unique to the Okta integration - this could happen with the SAML 
> connector as-is. 

## Staged release

This is suggested structuring of the work required, such that each step delivers
useful features with minimal re-work in later stages.

### Stage 1: Automation & Basic sync

 * Automated creation of SSO SAML connector
 * Limited Okta user sync
   * Polling Only
   * Teleport user created/destroyed in response to changes in Okta
   * **No** group memberships pulled from Okta (drastically reduces Okta API hit 
     count)
   * All users automatically get `access` and `requester` roles on creation.
 * Web UI treats okta users like local users (for example, Okta users may have 
   traits added/removed, user may be deleted - but will be re-created on next 
   reconciliation)

### Stage 2: Implicit access lists

I've included this before other, Okta-specific work because:
 * it is useful in its own right,
 * it enables the following Okta-specific work to be immediately useful, rather
   than having to wait for this to be implemented afterwards.

### Stage 3: Real-time updates
 * SCIM handling
   * Exposing SCIM endpoints
   * Handling user/group/app events by updating user traits to match
 * Including Okta Group membership in User traits
   * Fetching Okta group memberships for each upstream Okta during 
     reconciliation (requires at least 1 API call per group, more for groups
     with many (~200+) members)
   * Adding Okta group memberships as traits for each Teleport user
### Stage 4: Enhanced UI & Automatic access list creation
 * User presented with traits list during plugin installation
   * Common Okta traits exposed via querying Sync Server 
 * Automatic creation of Implicit Access Lists based on selected traits.
