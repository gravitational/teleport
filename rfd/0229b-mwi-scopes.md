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
existing constructs that exist for Scoped RBAC and MWI.

### User Stories

Personas:

- Cluster Admin: A global administrator of the Teleport cluster. Typically, a
  member of the team that owns and operates the Teleport installation.
- Scope Admin: An administrator of a specific scope within the Teleport
  cluster. Typically, someone who owns infrastructure that belongs to a group
  or team within the organization.

wipwipwip.

### Behaviour

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

The Scope Admin then assigns Scoped Roles to the Scoped Bot through Scoped Role
Assignments or via Scoped Access Lists. They cannot assign unscoped Roles to the
Scoped Bot.

TODO: Do the Scoped Roles assigned to the Scoped Bot need to be within the same 
scope or a sub-scope of the scope in which the Scoped Bot exists? Or do we
permit a Scoped Bot existing in `/foo` to be assigned Scoped Roles/privileges in
`/bar`? I think there's some merit in this - e.g allowing a scope admin to grant
an NHI owned/operated by another team to access resources within their scope.

The Scope Admin then creates a Scoped Join Token to allow `tbot` to join as 
the Scoped Bot. The Scoped Join Token MUST be created within the same scope as 
the Scoped Bot.

## Implementation Details

## Security Considerations

## Future Improvements

### Sub-scoping of generated credentials

## Appendix A: Decisions

### A.1: The mechanism of Bot scoping