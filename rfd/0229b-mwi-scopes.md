---
authors: Noah Stride (noah@goteleport.com)
state: draft
---

# RFD 0229b - Scopes: Machine & Workload Identity

## Required approvers

- Engineering: (@fspmarshall || @rosstimothy) && @timothyb89
- Product: ?

## What

Extend Machine & Workload Identity with support for the recently-introduced
Scopes feature. This will include adding scopes support to the configuration
mechanisms for MWI (e.g. the Bot) and support within `tbot` itself for correctly
issuing scoped certificates.

At this time, Scopes only has support for a subset of Teleport's features, and,
as such the design for MWI Scopes will be limited to those features.

## Why

There are largely two key motivations to implementing Scopes for MWI:

- To provide consistency of experience: Users of Teleport who are leveraging 
  scopes will want to be able to leverage them for machines as well as their
  humans.
- To resolve challenges with managing MWI in large organizations.

The first element is fairly self-explanatory, however, it is worth diving a 
little deeper into the problems we've seen with MWI in large organizations.

In a large organization, we usually see users of Teleport broken down into two 
categories: those who manage the Teleport infrastructure itself, and, those
who consume it for access to resources. This latter group often owns their
own infrastructure and these teams may wish to onboard MWI for Non-Human 
use-cases (e.g. CI/CD).

Unfortunately, granting these teams the ability to self-manage MWI configuration
resources isn't feasible due to the global, high-level of privilege this infers:

- Bot: A user who can create a Bot can assign it any role, including roles which
  may grant access to sensitive resources that are not owned by the user who is
  creating the bot. As the user can then join a Bot to receive credentials for
  these roles, they can effectively access any resources within their
  organization. This may be accidental or malicious.
- Join Token: To be able to join a Bot, a user must be able to create a join
  token. However, this privilege is not granular - the ability to create a join
  token permits a user to create a join token for any Bot (or indeed, other
  types of resource like a Proxy). This again provides a significant opportunity
  for privilege escalation.

Because of this, the configuration of MWI falls on the team that owns the
Teleport infrastructure itself. In smaller organizations this may be acceptable,
but in large organizations, this becomes a significant bottleneck for teams that
wish to onboard MWI and can even discourage its adoption.

To resolve this problem, we need to create a safe way for to delegate the
ability to onboard bots to teams that will leverage them.

## UX

This section explores the behavior and user experience of Scoped MWI without
delving into implementation specifics. It expects some familiarity with the 
existing constructs of Scoped RBAC and MWI.

### User Stories

Personas:

- Cluster Admin: A global administrator of the Teleport cluster. Typically, a
  member of the team that owns and operates the Teleport installation.
- Scope Admin: An administrator of a specific scope within the Teleport
  cluster. Typically, someone who owns infrastructure that belongs to a group
  or team within the organization.

wipwipwip.

### Behaviour

This section summarizes the expected behaviour of Scoped MWI from a Scope
Admin's perspective.

#### Creating the Scoped Bot

A Cluster Admin grants the Scope Admin the ability to create, read, update and 
delete Scoped Bots and Scoped Join Tokens within their scope through a Scoped
Role and Scoped Role Assignment.

The Scope Admin can then create the Scoped Bot. To do so, they must specify:

- A name for the Scoped Bot. This name must be unique globally within the
  cluster, across both scoped and unscoped bots.
- The scope that the Scoped Bot should exist within. This must be a scope, or a 
  subscope of a scope, that they have permission to create Scoped Bots within.

Notably, at this stage, they do not specify roles as they would do with an
unscoped Bot. It is not possible to create a Scoped Bot with unscoped Roles.

#### Assigning the Scoped Bot privileges

The Scope Admin then assigns Scoped Roles to the Scoped Bot through Scoped Role
Assignments or via Scoped Access Lists. They cannot assign unscoped Roles to the
Scoped Bot.

The Scoped Roles assigned to the Bot must exist within the same scope as the Bot,
or a child scope, and they must be assigned within the same Scope as the Bot or
a child scope. This negates the possibility of cross-scope privilege assignment.

The Scope Admin then creates a Scoped Join Token to allow `tbot` to join as 
the Scoped Bot. The Scoped Join Token MUST be created within the same scope as 
the Scoped Bot.

#### Joining the Scoped Bot

The Scope Admin can now setup `tbot` on a machine to join as the Scoped Bot
using the Scoped Join Token. To do so, they must configure:

- The name/method of the Scoped Join Token.
- The address of the Auth or Proxy Service.
- A type of service they would like `tbot` to run (e.g `identity`).

The certificates that `tbot` receives upon joining and that it generates for
services will be pinned to the scope in which the Scoped Bot exists.

## Implementation Details

## Security Considerations

## Future Improvements

### Sub-pinning of output credentials

Whilst the credentials of a scoped Bot may be pinned to a specific scope or 
global, it may be useful for users to further constrain the access of these
credentials by pinning them to a subset of what the Scoped Bot can access.

This provides an opportunity to reduce the blast radius by allowing users to 
produce credentials with the least privilege necessary for a given task.

This should be viewed as a surface-level constraint. If a bad actor is able to
gain root access to a machine, then they will be able to extract the bot's 
internal credentials and use these to issue credentials without this
sub-pinning. This is an important consideration for those who would use this
feature.

### Cross-scope privileges

Challenges:

- We propose making tbot credentials are pinned to the scope the Scoped Bot
  exists in - so whilst the Bot could be assigned cross-scope privileges, it
  won't actually be able to use them?
- How do we bind the Scoped Role Assignment to the Bot & Scope so the deletion
  of the Bot doesn't allow someone to "swoop" in and steal the sra from another
  scope?

## Appendix A: Decisions & Thinking

This section exists as a record of my thinking whilst researching and writing 
this RFD. It should not be considered a canonical part of the design, but, may
help provide context around my thought process and decisions for future readers.

### A.1: The form of Scoped Bots

When it comes to introducing the concept of a Bot that is scoped, we have a
number of options as to what form this actually takes. Broadly, there are three
options.

Add a scope field to the existing Bot resource:

- Perhaps the most "straight-forward" implementation. No need to introduce new
  `tctl` commands, IaC resources or RPCs. This results in the least
  "maintenance" burden for the team going forward.
- Introduces more complexity to the existing Bot resource - both in terms of 
  implementation and UX. For example, certain fields (e.g `roles`) will not be
  permitted when `scope` is specified.

Introduce a new resource type - ScopedBot:

- This provides a clear, new, separate schema for the user to interact with.
- Introduces tough questions around how we handle ScopedBots in the places
  we handle Bots today. For example:
  - How do we encode this ScopedBot identity into a TLS certificate? Do we
    re-use the BotName field, or introduce a new field (ScopedBotName)? Do 
    we introduce new UserKind?
  - How do we present ScopedBots in UX? Separate list page? Intermingled?
  - How do we handle this in analytics/usage reporting? Today we use the Bot's
    name to identify analytics, we'd need to introduce a new field to
    distinguish between a Bot and ScopedBot with the same name - or prohibit
    them holding the same name.
  - Arguably - these tough questions could be a "good" thing. We'd be forced to
    consider all places individually rather than it "automatically" working with
    ScopedBots. Whilst it increases the amount of work, it decreases the
    likelihood of a mistake.
  - However, solving all of these problems again for ScopedBots significantly
    increases the amount of work required to implement and maintain into the 
    future.
- Requires us to introduce and maintain wholly separate RPCs, `tctl` commands
  and IaC resources for ScopedBots.


Introduce a subkind of the existing Bot resource:

- Potential to abstract over Bot (normal) and Bot (scoped subkind) with a single
  interface in places where we want to treat both the same way. Although this
  may be fairly contrived.
- Not particularly feasible to re-use existing RPCs that we have today. We'd 
  either need to introduce V2 RPCs that can return normal or scoped - or
  introduce standalone RPCs for scoped Bots.
- Introduces some fairly awkward complexity to listing and fetching since the
  schema for the subkind will be "incompatible" with the normal Bot schema.

The subkind route appears to offer many of the drawbacks of both implementations
and few of the benefits of either. This leaves the choice between adding a 
scoped field to the existing Bot resource or introducing a new ScopedBot 
resource.

### A.2: The Wrong Implementations

When trying to determine the path for a complex implementation, it can be
helpful to consider first what *not* to do. This section explores several
"wrong" implementations of Scoped MWI and why they are not suitable.

#### A.2.1: The Lazy Way

The laziest or most naive implementation of Scoped MWI would be to:

- Not scope Bots or their join tokens at all. Leave these parts of the
  implementation as is.
- Allow Bots to be assigned Scoped Roles via Scoped Role Assignments or Scoped
  Access Lists.
- Build new RPCs for `tbot`'s certificate issuance flow that can calculate the
  roles that `tbot` can access based on Scoped Roles/Scoped Access Lists.

This implementation falls short in a number of ways:

- It doesn't solve the problem of allowing the creation of Bots and Join Tokens 
  to be safely delegated to teams who will consume them. Bots and Join Tokens
  would still need to be created by Cluster Admins, even if then the Scope Admin
  would be able to assign the Bot privileges themselves.
- Whilst appearing simple, it actually introduces significant complexity in the
  cognitive model of MWI. Bots would be able to hold scoped and unscoped
  privileges, and, the implementation of `tbot` and the Auth Server would need
  to account for this duality. There would be no clear "mode" to operate in.