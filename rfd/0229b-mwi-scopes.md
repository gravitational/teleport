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

To resolve this problem, we need to create a safe way to delegate the ability to
onboard bots to teams that will leverage them.

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

wip: Collect user stories from customer, anonymize and summarise.

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
Scoped Bot as this would allow the Scope Admin to escape the confines of their 
scope.

There are a number of rules that govern the assignment of privileges to a Scoped
Bot. Some of these rules are inherited from the Scopes RFD (marked RFD229) and
some of these rules are introduced by this RFD and specific to Scoped Bots
(marked RFD299B).

These rules are as follows:

- RFD299-1: A Scoped Role is only assignable at the scope of the Scoped Role or
  a descendent scope.
  - nb: A Scoped Role may include an `assignable_scopes` field that further
    constrains where it may be assigned.
- RFD229-2: A Scoped Role Assignment's scope of effect must be the same or a
  descendent of its scope of origin.
- RFD229B-1: The scope of origin and scope of effect of a Scoped Role Assignment
  for a Scoped Bot must be the same scope or descendent scope of the Bot's scope.
  - nb: RFD229-2 constrains scope of effect to at most scope of origin for SRA.

In practice, these rules mean that a Scoped Bot can only be assigned privileges
that would allow it to access resources within its own scope or in descendent
scopes. For example, a Scoped Bot in `/foo/bar`:

- Could be permitted to access resources in `/foo/bar` or `/foo/bar/buzz`.
- Could not be permitted to access resources in `/foo`.
- Could not be permitted to access resources in `/zip`.

These rules are backed up by the Scope Pinning mechanism. Certificates issued to
the Scoped Bot are pinned to the scope in which it exists. This prevents access
to any scoped resources outside the pinned scope.

This key constraint of the bot's scope of access to its scope of origin is
designed to simplify the initial design without introducing the risk of scope
isolation being compromised. [A.1: The scoping of Scoped Bots](#a1-the-scoping-of-scoped-bots)
explores this decision in depth and [Future Improvement: Cross-scope privileges](#b2-cross-scope-privileges)
explores relaxing this constraint in combination with a series of additional
controls.

Worked Examples:

| Bot Scope | SR Scope | SRA Scope of Origin | SRA Scope of Effect | Commentary                   |
|-----------|----------|---------------------|---------------------|------------------------------|
| /a/b      | /a/b     | /a/b                | /a/b                | OK                           |
| /a/b      | /a/b/c   | /a/b/c              | /a/b/c              | OK                           |
| /a/b      | /a       | /a/b                | /a/b                | OK                           |
| /a/b      | /a/b     | /a/b/c              | /a/b/c              | OK                           |
| /a/b      | /a/b     | /a/b                | /a/b/c              | OK                           |
| /a/b      | /a/b     | /a                  | /a                  | NOT OK - RFD229-1            |
| /a/b      | /a/b     | /a/b                | /a                  | NOT OK - RFD229-1 & RFD229-2 |
| /a/b      | /a       | /a                  | /a                  | NOT OK - RFD229B-1           |
| /a/b      | /z       | /z                  | /z                  | NOT OK - RFD229B-1           |

#### Joining the Scoped Bot

The Scope Admin then creates a Scoped Join Token to allow `tbot` to join as
the Scoped Bot. The Scoped Join Token MUST be created within the same scope as
the Scoped Bot.

The Scope Admin can now setup `tbot` on a machine to join as the Scoped Bot
using the Scoped Join Token. To do so, they must configure:

- The name/method of the Scoped Join Token.
- The address of the Auth or Proxy Service.
- A type of service they would like `tbot` to run (e.g `identity`).

### Example Full Configuration

First, the configuration used to grant the Scope Admin the ability to manage
Scoped Bots, Join Tokens, Scoped Roles and Scoped Role Assignments:

```yaml
kind: scoped_role
version: v1
metadata:
  name: staging-admin
scope: /staging
spec:
  allow:
    rules:
      - kind: bot
        verbs: [create, read, update, delete]
      - kind: scoped_role
        verbs: [create, read, update, delete]
      - kind: scoped_role_assignment
        verbs: [create, read, update, delete]
      - kind: scoped_token
        verbs: [create, read, update, delete]
---
kind: scoped_role_assignment
version: v1
metadata:
  name: alice-staging-admin
scope: /staging
spec:
  user: alice
  assignments:
    - role: staging-admin
      scope: /staging
```

With these privileges, Alice can now create the Scoped Bot and Scoped Join
Token:

```yaml
kind: bot
version: v1
metadata:
  name: staging-deployer
scope: /staging
spec: {}
---
kind: scoped_token
version: v1
metadata:
  name: staging-deployer 
scope: /staging
spec:
  assigned_scope: /staging
  bot_name: staging-deployer
  roles:
    - Bot
  join_method: token
  mode: single_use # wip: what to do with mode??
```

Now, Alice can create a Scoped Role and assign it to the Bot using a Scoped
Role Assignment:

```yaml
kind: scoped_role
version: v1
metadata:
  name: staging-ssh-access
scope: /staging
spec:
  allow:
    # Grants access to all SSH nodes as root within the assigned scope.
    node_labels:
      "*": "*"
    logins:
      - root
---
kind: scoped_role_assignment
version: v1
metadata:
  name: staging-deployer-ssh-access
scope: /staging
spec:
  user: staging-deployer # wip: do we add a new "bot" field here to avoid confusion?
  assignments:
    - role: staging-ssh-access 
      scope: /staging
```

## Implementation Details

**wip wip wip**

### Bot Resource

**wip wip wip**

wip: break this section up into sub-sections and add more detail where any
wip: new fields will be added.?

A new `scope` field will be added to the root of the Bot resource. When this 
field is provided, the Bot is considered a scoped Bot. This determines the scope
that the Bot exists within for the purposes of administration and the scope to
which the Bot's credentials are pinned to limit its scope of effect.

Creating, reading, updating and deleting scoped Bots will occur via the normal
Bot RPCs (and hence `tctl` commands and IaC resources). However, there are a 
number of differences in behavior when a Bot is scoped vs unscoped.

Firstly, access control for CRUD of the Bot resource will instead by governed
by scoped access checks. For creates and deletes, the implementation is fairly
trivial.

For the Get RPC, fetching a scoped Bot for which you do not have access will
yield a NotFound error. This is preferred to a PermissionDenied (or similar)
which leaks the existence of the Bot.

For the List RPC, scoped Bots for which you do not have access will be filtered
from the results. Additionally, support will be added for filtering by exact
scope or descendent scope. This will enable by-scope UI and `tctl` CLI commands.

We must pay particular attention to the Update and Upsert RPCs since these
allow a Bot to be transitioned from unscoped to scope, scoped to unscoped, or
between scopes. This introduces a significant risk of privilege escalation
or compromise of scope isolation if not handled correctly.

To mitigate this risk, we will not permit updates or upserts that change the 
`scope` field of an existing Bot within the initial iteration. This will be
rejected with a PermissionDenied error. In a future iteration, we may consider 
relaxing this restriction with additional controls.

wip wip wip.

The following additional validation will be enforced:

- The `scope.roles` field must not be set.

#### UX Changes

tctl / UI. -- probably shift these into the UX section and out of implementation
details.

### Bot Instances

wip: we likely need to implement same rbac changes for Bot Instances as well.
wip: we will need to propagate scope field into Bot Instance for the sake of
wip: performance otherwise we'll need to look up bot for bot instance. also
wip: probably some security repercussions here to consider. also ux for tctl
wip: and ui needs to reflect scoped searching?

### Scoped Role Assignments

The ScopedRoleAssignment resource will be extended with a new field, `spec.bot`,
which will be used to specify a Bot as the target of a role assignment instead
of a user.

The `spec.bot` field will be mutually exclusive with the `spec.user` field.

As per [Behaviour: Assigning the Scoped Bot privileges](#assigning-the-scoped-bot-privileges),
new validation rules will be added to the Create/Update/Upsert RPCs for the 
ScopedRoleAssignment to enforce the following when `spec.bot` is set:

- That `spec.bot` references a bot that exists.
- That `spec.bot` references a bot with a scope.
- That the scope of origin (`scope`) and scope of effect
  (`spec.assignments.*.scope`) of the role assignment are the same or descendent
  scopes of the bot's scope.

This validation must only be performed on creation or update, and should not
be enforced on read of existing role assignments - this avoids the risk of
breaking reads/cache initialisation if the condition of the referenced bot
changes.

Instead, the privilege calculation mechanisms defined under
[Bot Identity Representation and Certificate Issuance](#bot-identity-representation-and-certificate-issuance)
should ignore any invalid scoped role assignments that would violate the above
rules. Ignoring role assignments is safe as scoped roles cannot include deny
clauses and thus ignoring a role assignment can only lead to a decrease in
privileges.

### Joining

Today, unscoped bots join using unscoped Join Tokens via the standard Join RPCs.
The Join Token has `spec.roles` set to `["Bot"]` and the name of the Bot bound
to the token in `spec.bot_name`.

Scoped agents join via the same RPCs, but using Scoped Join Tokens. This is a 
distinct resource that includes many of the same fields, but, also includes a 
a `scope` and `spec.assigned_scope` field. A Scoped Join Token can only be
created with `spec.assigned_scope` that is the same as `scope` or a descendent
of `scope`.

Joining for scoped Bots will be similar to unscoped Bots and Scoped agents. A
Scoped Join Token will be used with `spec.roles` field set to `["Bot"]` and the
name of the scoped Bot provided in a `spec.bot_name` field. As this 
`spec.bot_name` field does not currently exist, it will need to be added to the 
ScopedToken resource.

The following new fields will be introduced to the ScopedToken resource:

- `spec.bot_name` (string): The name of the scoped Bot that is joining. This
  must be set when `spec.roles` includes `Bot` and must not be set otherwise.

The following new validation will be enforced for the ScopedToken resource:

- When `spec.roles` includes `Bot`:
  - `spec.roles` must have a length of 1. That is, other roles cannot co-exist
    with the `Bot` role.
  - `spec.bot_name` must be set.
  - `spec.assigned_scope` and `scope` must be set to the same scope, and this 
    scope must be the scope of the scoped Bot.
- When `spec.roles` does not include `Bot`:
  - `spec.bot_name` must not be set.

When joining with an unscoped token, the following new validation will be
enforced:

- The Bot must not have a scope set.

When joining with a scoped token, the following new validation will be enforced:

- The Bot must have a scope set, and this scope must match the
  `spec.assigned_scope` and `scope` of the token.
  - This is a critical control for ensuring the isolation of scopes is not 
    compromised - i.e. an admin in scope `/foo` cannot create a join token for
    a scoped bot in `/bar`.

wip: What do we do about the `mode` field on the join token when used for 
wip: Bots. "single_use" clashes with bot renewal mechanisms. "unlimited" implies
wip: weird behaviour for bots using the `bound_keypair` join method. Perhaps
wip: we just require this field is empty for bot scoped join tokens.

In the certificate generation process that occurs upon successful joining,
there is one key difference: the resulting certificate must be pinned to the
scope that the scoped Bot exists within.

#### Auditing

The `bot.join` audit log event will be extended with new fields to capture 
information relevant to the scoped Bot joining process:

- `scope` (string): The scope of the Bot that has joined. Unset for unscoped 
  joining.

The `scoped_token.created` audit log event will be extended to capture new
fields:

- `bot_name` (string): The name of the Bot that this scoped token is associated
  with. Unset for non-Bot tokens.

### Bot Identity Representation and Certificate Issuance

wip, wip, wip

wip: Remark on how bot identities work today - internal and assumed roles.

We issue certificates for bot instances via three different paths:

- `tbot` successfully calling of a Join RPC, we issue the Bot's "internal"
  credentials. These "internal" credentials reflect the internal Bot Role rather
  than any roles which are assigned to the Bot.
- `tbot` calling the GenerateUserCerts RPC to generate certificates intended for
  services/outputs. This RPC is called using the bot's internal credentials and
  the resulting certificate reflects the Bot's assigned roles via the role 
  impersonation mechanism.
- Special case - renewal for `token` join method bot instances. `tbot` calls the
  GenerateUserCerts RPC using the bot's internal credentials with the intent of
  producing internal credentials that expire at a later time than the current
  credentials. This triggers a set of special renewal checks.

Likely, all three of these paths will need to be modified in some way to support
the scoping of bots. 

wip: Lightly remark on how scoped roles are encoded, and the meaning of this,
wip: and how this clashes with the way role assumption works today.

#### Calculating Scoped Role Assignments

wip: Re-iterate rules that must be followed when calculating the roles that
wip: should be encoded for a bot.

#### GenerateUserCerts RPC

wip: GenerateUserCerts will need to be modified to ensure certificates issued to
wip: scoped bots take into account SRAs, and, reject the use of role
wip: impersonation. There may be hidden complexity here as GenerateUserCerts 
wip: assumes that a Bot will be leveraging role impersonation for output
wip: certificates, and, will treat unassumed GenerateUserCerts for Bots as an
wip: attempt to perform a internal certificate renewal (which is only valid for
wip: bots using the `token` join method...)

### `tbot`

wip: `tbot`'s mechanism for requesting credentials will need to be aware of its
wip: scoped status and avoid using role impersonation plus take into account
wip: any changes in Certificate Issuance.

### Implementation Phases

wip: list of individual tickets we will create as part of epic, and rough order
wip: of implementation

## Security Considerations

The prime security consideration throughout this design and implementation is
ensuring that the isolation guarantees provided by scopes cannot be circumvented
through the use of scoped MWI. That is, that an individual with administrative
privileges within a scope, cannot leverage scoped MWI to gain access to
resources outside of that scope.

wip: summarize all controls in place, and where those controls are defence in 
depth or accounting for different attack vectors.

## Appendix A: Decisions & Thinking

This section exists as a record of my thinking whilst researching and writing 
this RFD. It should not be considered a canonical part of the design, but, may
help provide context around my thought process and decisions for future readers.

### A.1: The scoping of Scoped Bots

A rather early philosophical question revolves around whether a Bot's scope of
origin should constrain its scope of privilege. That is, whether a Bot in
`/foo/bar` should theoretically be capable of accessing resource in `/foo` or 
indeed `/zip` if an admin so desires.

#### User Experience

First, let's approach this from the perspective of possible users and consider
what use-cases do and do not require this ability.

Use-cases that work even when a Scoped Bot's privilege is constrained to its
scope of origin:

- Engineering team implementing a CI/CD pipeline to deploy to infrastructure
  that they own within their scope. Their Scope Admin can create the Bot, Join
  Token, SR and SRA all within their scope alongside their infrastructure.
- Engineering team deploys an AI agent that can access their infrastructure
  that they own within their scope.
- Engineering team deploying Terraform IaC to manage the configuration of their
  scope. The Scoped Bot/SR/SRA can be bootstrapped within their scope or within
  a parent scope (with the SRS granting access only to the team's child scope).

Within these use-cases, there is a unifying theme that the team's bot is only
accessing or managing infrastructure resources that belong to that team. The
isolation model actually works quite well here - avoiding accidentally granting
excessive privilege to the bot.

But, let's examine a counter example of a use-case that could require 
a scoped bot to have privileges outside its scope of origin.

Within this organization, a central security team offers security scanning
(think Trivy) to engineering teams. This security scanning may be mandated by 
policy. These engineering teams own their own infrastructure and its placed into
scopes in which these teams have privilege.

If it were not for constraining a Scoped Bot's privileges to its scope of 
origin, a theoretical setup would look something like: the security team creates
a Scoped Bot and Scoped Join Token in their scope. The engineering teams then
assign the security team's Scoped Bot privilege within their scopes using SRs
and SRAs.

For the organization, this setup may be desirable because:

- The security team maintains their own Bot and Join Token for the security
  scanning tool. They can change how this Bot joins without needing to involve
  other teams across the organization.
- The security team has the least privilege necessary. They have not been 
  excessively granted privilege in the scopes owned by the engineering teams -
  those who own the infrastructure remain in full control of who can access it.

Within a design where a Scoped Bot's privileges are constrained to its scope of
origin, this security scanning bot would need to be created within each scope or
within a parent scope. This risks granting it excessive privilege if created
within a parent scope, or, creates significant maintenance overhead.

We can generalize from this example. In organizations where teams offer services
to other teams that own infrastructure, there is likely a desire for bots to be
able to access resources outside their scope of origin.

#### Security Concerns

The primary concern with allowing a Scoped Bot to have privileges outside its
scope of origin is the risk that we introduce avenues for an scope admin's
privileges to escape their scope.

A scoped admin of `/foo` cannot be allowed to directly or indirectly cause a bot
that they have created in `/foo` to have privileges outside the `/foo` scope. If
this were possible, then an attacker who has compromised the `/foo` scope admin
could leverage bots to circumvent scope isolation and expand the blast radius 
from `/foo` to the entire cluster.

Let's examine a few theoretical vectors of privilege escalation.

##### Traits & Role Templates

Traits are a set of metadata that can be associated with a bot or user. These
traits can be used within role definitions as part of a role template. This
allows for one role definition to grant different levels of privilege depending
on the traits of the bot or user that holds it.

The admin of a bot's scope of origin has the ability to modify the traits of the
bot. If the admin of another scope assigns the bot a role within their scope
that leverages traits with role templating, then the admin of the scope of
origin can cause the bot to have a different set of privileges than what may
have originally been intended by the admin of the other scope.

Notably, this escalation vector only exists where the bot has been assigned a 
role that leverages traits with role templating. Where the bot is assigned a
static role, this risk does not exist.

Potential mitigations:

- Document this risk and encourage scope admins to avoid the use of role
  templates.
- Ignore bot traits when evaluating roles outside the bot's scope of origin.
  Traits would function as expected within the bot's scope of origin. This
  does eliminate the risk of escalation, but is potentially confusing for 
  operators.
- Do not allow traits to be set for scoped bots.

##### Name Reuse

In the simplest design, an admin in one scope would grant access to a bot in
another scope by creating a role assignment that references the bot only by
name.

This opens the door for a potential name reuse attack:

1. Scope admin in `/foo` creates a bot called `bernard`.
2. Scope admin in `/bar` creates a role assignment that grants privileges in
   `/bar` to `bernard`.
3. Scope admin in `/foo` no longer has use for `bernard` and deletes the bot but
   does not have the ability to delete the role assignment in `/bar`. 
4. An attacker, who has compromised the `/zap` scope admin, creates a new bot
   called `bernard` in `/zap`. This bot now has the privileges in `bar` that 
   were intended for the original `bernard` bot.

Potential mitigations:

- Namespacing of scoped bots.
- Require that role assignments are bound to both the scoped bot name and its 
  scope of origin.
- Require that role assignments reference a unique identifier of the bot that is 
  not reusable (e.g. a UUID) instead of the bot's name.
  - This would be very unlike the rest of the way Teleport behaves and presents
    a significant UX challenge.

Notably, this risk is introduced due to how role assignment works in classic
RBAC vs scoped RBAC. In classic RBAC, role assignments are made as part of the
definition of the bot itself. In scoped RBAC, role assignments are separate
resources that reference the bot.

This risk is a good argument for considering namespaces within the designed of
Scoped RBAC (and Scoped MWI by extension). Whilst this is not currently the
case, there are ongoing discussions about whether namespacing should be added
to scopes.

##### Confused Deputy Privilege Escalation

Let's consider an automation which leverages a bot in `/security`. This 
automation polls a list of SSH nodes it has access to and then connects to them
and transfers some sensitive data to them.

The creators of this bot assigned it privileges using a Scoped Role and Scoped
Role Assignment in the `/security` scope, and intend it to only perform this
action against the SSH nodes in the `/security` scope.

In an implementation where a bot's scope of origin does not constrain its
privileges, this opens up the door to an interesting attack that leverages 
granting the bot additional privileges in an orthogonal scope:

1. A bad actor has compromised the `/staging` scope admin.
2. They onboard a new SSH agent into the `/staging` scope to which they have 
   access.
3. They create a Scoped Role that grants access to this SSH agent.
4. They create a Scoped Role Assignment that assigns this role in `/staging`
   to the Bot that exists in `/security`.
5. The automation, upon fetching a list of nodes, discovers this new node it
   has access to and transfers the sensitive data to it.
6. The attacker leverages their privilege within `/staging` to exfiltrate the
   data that has now been transferred to this node by the automation.

This exploit depends on the fact that the automation has been written to rely on
the bounds of privilege for filtering the nodes on which it should act. An
implementation of the automation that explicitly filtered by scope or by
node uuid would prevent this kind of attack.

However, this foot gun is far too easy of a trap to fall into, and considering
the kinds of automations that are in common use with MWI this poses a
significant risk.

The primary mitigation we propose here is to by-default constrain a Bot's 
privileges to its scope of origin and require explicit action to relax this 
constraint. We would implement a configuration option within `tbot` that allows
the operator to choose the scope to which the bot's credentials are pinned.
They may then explicitly choose to pin the bot's credentials to the root scope,
in order to generate credentials that leverage any privileges assigned to the
bot regardless of scope, or may choose some other orthogonal or parent scope.

In addition to this mitigation, we should publish recommendations that encourage
users to implement explicit filters in their automations and to avoid relying on
the bounds of privilege for filtering where the automation performs sensitive
actions.

##### Direction

Whilst it seems clear that there are use-cases that require bots to have
privilege outside their scope of origin, it is also clear that this has 
significant implications for the implementation and security model for 
Scoped RBAC.

For this reason, the initial implementation of Scoped MWI should constrain a
Scoped Bot's privileges to its scope of origin. This does not preclude us from
relaxing this constraint in future with the introduction of additional controls.

### A.2: The form of Scoped Bots

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

Arguably, implementing the initial MVP reusing the existing Bot resource with
a scope field is the quickest and simplest way route. There's already a strong
case for refactoring the Bot resource itself and its internal RBAC at a future
date, and, this could be tied into the future refactor to introduce the Scoped
Bot resource.

### A.3: The Wrong Implementations

When trying to determine the path for a complex implementation, it can be
helpful to consider first what *not* to do. This section explores several
"wrong" implementations of Scoped MWI and why they are not suitable.

#### A.3.1: The Lazy Way

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

## Appendix B: Future Improvements

### B.1 Sub-pinning of output credentials

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

From a user's perspective, they would specify the scope when configuring an 
output service within `tbot`:

```yaml
services:
- type: identity
  scope: /foo/bar
  destination:
    type: directory
    path: /opt/machine-id
```

The implementation of this would take the following form:

- wip wip

### B.2 Cross-scope privileges

One of the key constraints identified for the initial iteration is that a 
scoped Bot's access is constrained to its scope of origin
(or descendent scopes). The reasoning for this is to reduce the risk of 
violating scope isolation guarantees and to simplify the initial iteration. This
is explored in depth in [A.1: The scoping of Scoped Bots](#a1-the-scoping-of-scoped-bots).

As explored in that section, there are feasible use-cases that would be enabled
by relaxing this constraint. However, also identified are significant potential
security implications of doing so. To enable this safely, we would need to 
introduce additional controls.

Before any implementation begins on this work, due to the presented risks, it is
agreed that this must require an amendment to this RFD or the creation of a
supplementary RFD. The rest of this section lightly summarizes the changes that
would be explored in more depth as part of the design of this next iteration.

There are two controls that prevent a scoped Bot's privileges from escaping the
confines of its scope of origin that would need to be relaxed:

- The restriction of assigned scopes in a SRA for a scoped Bot to the Bot's 
  scope of origin or a descendent scope.
- The pinning of the Bot's credentials to its scope of origin.

We propose that to relax these controls, the following new controls would likely
need to be introduced:

- A global, cluster-wide enablement of cross-scope privileges for scoped Bots.
  This derisks the introduction of this functionality causing a regression in
  the isolation of scopes for users who do not wish to leverage it. After a 
  sufficient period of testing, it may be appropriate to enable this by default.
- Limit the effect of scoped Bot traits to the Bot's scope of origin.
  Optionally, allow admins to assign traits to Bots that only apply within their
  scopes.
- Namespace bots by scope of origin, or, require SRAs to reference the bot's
  name and scope of origin to mitigate name reuse attacks.