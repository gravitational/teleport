---
authors: Alan Parra (alan.parra@goteleport.com), Forrest Marshall (forrest@goteleport.com)
state: draft
---

# RFD 0193 - Access Control Decision API

## Required approvers

- Engineering: @rosstimothy

## What

This proposal describes a means of reworking teleport access-control decisions to remove the need to perform
primary access-control decisions on peripheral agents and to bring teleport's internals more in line with common
access-control best practices.

## Why

Access-control decisions in teleport are typically made at the agent, and often times a single operation
will incur many small access-control decisions over the course of its execution. This distributed and innervated
model of access-control decision making has some benefits. For example, early versions of teleport made it a
priority to have agents continue to be able to make access-control decisions even when the auth service was
unavailable for long periods of time. However, there are also serious drawbacks to the distributed and innervated
model:

- Scalability: Access-control decisions at the agent require that agents have access to all necessary information
in order to be able to make correct access-control decisions. This forces us to encode a large amount of otherwise
unnecessary information into certificates, and forces agents to cache all resources relevant to access-control
decision making (namely roles). In some larger clusters, role replication alone can account for a significant portion
of all load incurred after an interruption of auth connectivity.

- Compatibility: Because teleport access-control includes the ability to specify unscoped deny rules, there is no
sane way to represent many new access-control features to older agents s.t. they can continue to make sensible
decisions. We've typically had to resort to cumbersome and imperfect hacks that negatively impact user experience,
such as injecting wildcard deny rules into the caches of outdated agents to force them to deny any access that might
require knowledge of a new feature.

- Maintainability: From both the developer and user perspective, distributed and innervated access-control decision
making poses maintainability challenges. For developers, it is difficult to reason about or change access-control
features because the implementation is so spread out, and so many different teleport components rely on internal
details of the implementation. For users, in order to take advantage of a bug fix or new feature, they must often
update thousands of teleport installations. This is particularly worrisome in the case of security fixes, as
access-control decision making is a particularly sensitive part of teleport.

- Development Cadence: Due mostly to the aforementioned issues, it is very difficult to make any non-trivial changes
to teleport access-control decision making. Most major proposed changes to teleport access-control decision making
(e.g. scoped rbac) have never even reached the point of a working prototype. This isn't only because of the lack of
a clear abstraction boundary, but that is a very significant contributing factor.

## Details

### Key Terminology

- Policy Decision Point (PDP): A component responsible for making access-control decisions. Sometimes referred to as
an Access Decision Function in some literature.

- Policy Enforcement Point (PEP): A component responsible for enforcing access-control decisions. Sometimes referred to
as an Access Enforcement Function in some literature.

- Decision Request: A request sent to the PDP describing an action and requesting a decision on whether or not
the action should be allowed, and any necessary information needed to correctly enforce limitations upon the action.

- Permit: A single data structure encoding all necessary information for a PEP to correctly enforce a conditional/
parameterized allow decision. A permit is *not* a credential, and must not be persisted or reused outside of the
context of the action about which the decision was made.

- Denial: A data structure encoding all necessary information for a PEP to correctly enforce a deny decision. Typically
this is just a message, but some denial cases may eventually trigger more complex flows (e.g. MFA).

- Decision/Decision Response: A response from the PDP to a Decision Request, containing one of either a Permit or a Denial.

### Non-Goals/Limitations

- This proposal does not aim to make any teleport agent resilient to compromise of any control-plane element. Most notably,
a compromised teleport PDP (typically the proxy) *will* be able to send malicious decisions to agents. Since proxies are already
able to impersonate users (and auths can do *anything*) this isn't a regression, but it is worth calling out explicitly.

- This proposal will effectively kill any remaining ability for teleport agents to continue to serve requests when proxies are
unavailable. This is mostly already the case, but currently certain role configurations can allow direct dial teleport ssh agents
to continue to be usable by using an openssh client to direct dial the agent even when the control-plane is entirely unavailable.
This will no longer be the case.

### Overview

We will rework teleport's access-control decision making to have a well-defined boundary between Policy Decision Point
and Policy Enforcement Point. The goal of this rework will be to abstract as much complexity as possible into the PDP,
while attempting to minimize the need to actually change how or where enforcement is done today. The PDP/PEP boundary
will be representable as a gRPC API, but where possible we will continue to ensure that teleport does not make unnecessary
network round trips.

We will also establish a set of conventions intended to make the transition to using the new system as easy as possible,
and to ensure that it is as difficult as possible to accidentally misuse it either by misunderstanding or oversight.

### API

The PDP/PEP boundary will be represented as a gRPC API, with methods for the different kinds of decisions. Because teleport
as a whole is an integrated PDP/PEP, our decisions are often much more nuanced than a simple allow/deny. This is one of
teleport's greatest strengths, as this allows us to provide very granular controls over actions. It also means that many
of the decisions we make are very verbose. Take ssh access for example: an 'allow' decision for ssh access must include
parameters for agent forwarding, port forwarding, concurrent connection limits, BPF events, expired cert disconnect, locking,
file copying, etc. For this reason we will eschew any attempt at creating a "unified" method for decisions in favor of
implementing custom methods and types intended to meet the specific needs of specific decisions.

In order to ensure that the PDP API remains simple and usable we will establish a set of conventions for how decision methods
should be implemented and how types should be structured:

- All "allow" decisions will be structured as conditional allows, pending the application of limits/parameters. All allow decisions
will be communicated by a type of the form `<Action>Permit`, which will encode all limits/parameters to be applied by the PEP. Much of
the agent-side teleport logic that previously would have worked with some combination of certificate identity and role set will instead
take the appropriate permit structure as input.

- Decision Service methods will conventionally take the form `rpc Evaluate<Action>(Evaluate<Action>Request) returns (Evaluate<Action>Response)`.
For example, the method for evaluating server access would read `rpc EvaluateSSHAccess(EvaluateSSHAccessRequest) returns (EvaluateSSHAccessResponse)`.

- Response objects will contain a top-level `Decision` oneof field with `Permit` and `Denial` variants. The underlying permit/denial
types will conventionally be named `<Action>Permit` and `<Action>Denial`.

- Common `RequestMetadata`, `PermitMetadata`, and `DenialMetadata` types will be provided and should be included in all requests, permits,
and denials. These types will contain common fields like teleport version of PDP/PEP, dry run flag, etc.

- We will not generally try to write polymorphic abstractions over different request/decision/permit/denial types, but helpers will be
provided for basic permit/denial control-flow. For example, a `decision.Unwrap` function will be provided that converts decision responses
into the form `(Permit, error)` where `error` is a `trace.AccessDeniedError` built from the contents of the denial.

- A standard `Resource` reference types with fields like  `kind` and `name` will be provided and should be prefered in
those APIs where specifying individual resources or sets of resources is necessary. PDPs will use this reference to load
the appropriate resource from local caches.

- Two standard `Identity` types will be provided for use in communicating caller identity within decision requests. One
corresponding to standard teleport TLS cert format, and one to the standard SSH cert format. Eventually, we would like
the PDP to be able to make decisions based on a standardized user/subject reference similar to the `Resource`
reference, with most fields currently stored in the certificate instead determined at runtime. In practice, moving away from
teleport's dependence on complex certificate-based identities is non-trivial. In order to support an iterative transition, we will
reimplement our existing identity types as a protobuf type and use that as the standard subject identifier. Individual fields will be
transitioned from being cert-bound to being determined PDP-side in an iterative manner. 

Here is a truncated example of the PDP API for server access:

```protobuf
service DecisionService {

  rpc EvaluateSSHAccess(EvaluateSSHAccessRequest) returns (EvaluateSSHAccessResponse);

  // ...
}

message EvaluateSSHAccessRequest {
  RequestMetadata metadata = 1;

  SSHIdentity ssh_identity = 2;

  Resource server = 3;

  string login = 4;
}

message EvaluateSSHAccessResponse {
  oneof decision {
    SSHAccessPermit permit = 1;
    SSHAccessDenial denial = 2;
  }
}

message SSHAccessPermit {
  PermitMetadata metadata = 1;

  repeated string logins = 2;

  bool forward_agent = 3;

  google.protobuf.Duration max_session_ttl = 4;

  bool port_forwarding = 5;

  int64 client_idle_timeout = 6;

  bool disconnect_expired_cert = 7;

  repeated string bpf = 8;

  bool x11_forwarding = 9;

  int64 max_connections = 10;

  // ... (there's a lot more that needs to go here)
}

message SSHAccessDenial {
  DenialMetadata metadata = 1;

  // ... (this may eventually support MFA flows, etc.)
}

message SSHIdentity {
  // ... (will contain repr of standard teleport ssh cert identity)
}
```

With the above API, most server access enforcement logic can be reworked to respect the PDP/PEP boundary with minimal changes.
`AccessChecker.EnhancedRecordingSet()` becomes `permit.Bpf`, `AccessChecker.GetHostSudoerts(server)` becomes
`permit.HostSudoers`, etc.


### Cross-Version Compatibility

One of the core problems that we need to solve is how to ensure that version/feature mismatches between PDP and PEP are easy to
handle in an intuitive and correct manner. Failure to correctly handle cross-version compat can lead to serious security issues,
and the current system of injecting wildcard deny rules at the caching layer is coarse, leads to false-positive denials, and is
cumbersome to implement correctly.

For most teleport API interactions where cross-version compat matters, we simply share the version
of the client with the server and vise-versa and make decisions based on that. This quickly becomes unwieldy, however,
when features start getting backported to preceeding major versions. Some APIs now use feature flags, which are much easier to work
with and backport, and are generally quite reliable. However, using either poses problems for a PDP API because some decisions need
to be made in contexts where the version of the PEP may not be known with 100% certainty. For example, decisions made on the proxy will
rely on details from the agent heartbeat, which may be outdated.

To solve this, permits will contain "feature assertions", which will specify which features *must* be available for the permit to be
properly enforced. Enforcing agents will then reject otherwise allowable actions if the permit contains one or more unknown features.
Feature assertions will be represented as a repeated enum for efficient encoding. Because feature assertions can be appended to permits
on a case-by-case basis, we minimize false-positives (instances where access gets denied due to an outdated enforcement point even though
the specific access did not merit the use of any new controls).

In general, the decision flow for deciding how to handle back compat across the PDP/PEP boundary will be as follows:

- If possible, implement new features s.t. it is safe for older PEPs to ignore them (any new feature that can be modeled 
as strictly increase privilege can fall under this bucket).

- If not safe to ignore, implement new features s.t. relevant controls can be enforced at the control-plane or via existing permit fields
(abstractions like scoped RBAC, updated server label selector syntax, and similar all meet this criteria).

- If none of the above, add a new feature assertion enum variant and update PDP logic to append the new feature assertion to permits if and
only if the specific permit in question requires the new controls be enforced (since said new controls will almost certainly necessitate a
new permit field, decided when to append the feature assertion should be trivial in most cases).

With the above decision flow/procedures observed, new access-control features should generally be safe to backport just like any other
(from a cross-version API compatibility perspective, backend resource versioning is outside the scope of this RFD).

To better illustrate the above flow, lets take a look at some hypothetical new features and how they might be tackled:

**Example 1: Local/Remote Port Forwarding Controls**

Say that we wanted to implement a new SSH access RBAC control that allowed administrators to specify wether to allow
local/remote port forwarding specifically rather than the current all or nothing `port_forwarding` control. Step one is
to decide if there is a way we can model this new control as something that can be safely ignored (e.g. by modeling it as
a privilege increase). Since the current `SSHAccessPermit` represents the ability to perform port forwarding as a single
boolean field, we can just add two new boolean fields (`local_forwarding` and `remote_forwarding`) and set them to true
as-needed, leaving the legacy forwarding field false unless local and remote are both true. In this manner, we can be
certain that outdated agents will simply fail to grant the new privilege without granting any unintended privileges.
No further compat work is needed.

**Example 2: Server Access Rate Limit**

Say we want to introduce a global rate limit control for server access attempts (e.g. prevent a user from performing more
than 10 ssh attempts per minute across the cluster). Step one is to decide if we can model this new control as something that
can be safely ignored. Since we don't have an older more strict variation of this behavior to fallback on, we cannot. Step two
is to decide if we can enforce the control at the control-plane. This we can do. Since the control-plane is always inline somewhere
during an ssh access attempt, we can enforce the rate limit by injecting deny decisions for all attempts that exceed the limit
(note: care needs to be taken to ensure we ignore "dry run" queries, like those from `tctl` for this kind of control).

**Example 3: BPF Killswitch**

Say we want to introduce a feature that causes teleport to kill ongoing ssh sessions if certain BPF events are detected associated
with that session (e.g. a `session.command` event whose path doesn't match some allow regex) and generate a notification for admins
to come investigate. Step one is to decide if we can model this new control as something that can be safely ignored. Since there isn't
an existing less-permissive behavior that makes a sensible fallback, we cannot. Step two is to decide if we can enforce the control
at the control-plane. BPF recording does not currently work in a manner that would make this practical. This leaves us with one option:
we need to ensure that outdated agents don't handle ssh access attempts that require the new control. To achieve this, we would do two
things. First, we would add a new `ENFORCEMENT_FEATURE_BPF_KILLSWITCH` variant to the `EnforcementFeature` enum. Second, we would update
the `EvaluateSSHAccess` method to append the new feature assertion to the permit's metadata whenever the permit contains a non-empty
`bpf_killswitch` field. With these done, we can safely backport the feature like any other and be confident that outdated agents will
not allow users to escape the control.

### Phased Implementation

The switch to PDP will be accomplished in two distinct phases. An "refactor" phase, and a "relocate" phase.

#### Refactor Phase

The refactor phase will consist of initial implementation of the PDP API, without any outward changes that might affect
cross-version compatibility. In order to seamlessly transition from decisions being made at agents to decisions being
made at the control plane, the implementation of the PDP logic will be polymorphic over two possible operating modes
depending on where it is running.

On the control plane, the PDP will have access to the entire set of teleport users and roles. This variant will be able
to serve as the backing of the gRPC API, and make internal decisions as if it were a remote PDP. This PDP will be a shared
service initialized once at process startup.

On agents, a local ephemeral PDP will be able to be initialized with a fake user store only knowing the identity of the
user making the request. This variant of the PDP will need to be initialized for each incoming request, and will be able to
only serve requests related to the user for which it was initialized (much like the `AccessChecker` today).

Common PDP logic will be shared between the two variants, meaning that so long as we need to support the agent-local PDP,
the control-plane PDP won't be able to make use of any state that cannot be derived from a combination of local agent cache
and user certificate identity. The APIs of both local and remote PDP will conform to the same interface so that any logic
relying upon the PDP can abstract over local and remote implementations.

During this phase we want to start moving as many elements as possible from being part of the certificate-derived identity to
being calculated on-demand within the PDP itself. As a prerequisite to this, we will need to implement a custom access request
cache, as access requests have a signficant impact on many identity fields. A custom cache will be necessary in order to handle
the fact that access requests are often used immediately upon approval, before approval can be relied upon to be propagated
to caches naturally.

The intent of the above work is to allow us to make the vast majority of necessary code changes in a totally backwards-compatible
manner and to backport them to all active release branches. This will ensure that ongoing work related to policy enforcement
doesn't need to reimplement the same logic for the old model when backporting. Though its worth considering trying to land
all major changes prior to a testplan so that we can get robust manual testing of all major features with the changes in
place before backporting.

To ensure that we've fully broken agent dependence on roles outside of the PDP, we will split the agent's local cache and api client
up s.t. all components other than the PDP will be statically prevented from accessing roles.

During this phase we will also implement tctl commands for directly invoking the DecisionService gRCP API on the control
plane. The exact syntax of these commands is TBD, but for simplicity we will try to mirror the grpc API as much as possible,
with the noteable exceptions that we will provide means of shorthand reference to identities and resources.  Ex:

```shell
$ tctl decision-service evaluate-ssh-access --username=alice@example.com --login=root --server-id=ba4dbf31-cc42-4766-8474-efc6b70aec80 --format=json
{
  "Decision": {
    "Permit": {
      "metadata": {
        "pdp_version": "v1.2.3"
      },
      "forward_agent": true,
      "max_session_ttl": "1h",
      "port_forwarding": true,
      ...
    }
  }
}

$ tctl decision-service evaluate-ssh-access --username=alice@example.com --login=root --server-id=4681f1b8-54ef-45bb-a62f-2c5ab60a88b5 --format=json
{
  "Decision": {
    "Denial": {
      "metadata": {
        "pdp_version": "v1.2.3",
        "user_message": "access denied to server 4681f1b8-54ef-45bb-a62f-2c5ab60a88b5"
      },
      ...
    }
  }
}
```

#### Relocate Phase

The relocate phase will see us actually transition to all access-control decisions being made at the control plane. The method
of relocation will depend on the kind of teleport agent. For tunnel agents, we will start performing denial at the control plane
itself, and allowed dials will have the Permit object forwarded to the agent as part of the dial. For direct dial agents, the
agent will call-back into the control plane to get a decision.

We intend to open a second follow-up RFD when we are closer to the relocate phase in order to explore it in more detail, but the
highlights are these:

- Agent reverse tunnel and proxy peering protocols will need to be updated to allow a trusted and replay-resistant permit message to
be sent from proxy to agent as part of an incoming dial.

- The mechanism by which trusted cluster dials are performed will need to be reworked s.t. all routing decisions are made at
the leaf proxy rather than the root proxy since access-control decisions will now get made as a part of routing. This will include
sending sideband information about user identity when forwarding the dial from root proxy to leaf proxy in order to ensure that
the leaf proxy can correctly map the identity without relying on user certificate information.

- One major version of buffer will be needed during which agents still cache roles locally and roles/traits are still applied to certs
so that we can fallback to local decisions when talking to outdated control plane elements.

- Agentless/openssh nodes are already using a "decisions at control plane" model, but they still depend on logins being present in
certificates. We may want to consider a mechanism for eliminating logins at the same time (e.g. by lazily provisioning certs containing
only the target login at the proxy).

### Questions/Alternatives/Ongoing Research

- Some protocols (e.g. k8s) don't map as cleanly to a single decision being made in advance at dial time. We are still looking into our
options there and plans related to such protocols are subject to change. SSH being the most widely used protocol, and the protocol that
incurs by far the most role replication load, has been the focus of our investigation thus far. It may end up being that
some protocols' permits end up being something more akin to a compiled "resource selector" that can be sent as part of a dial. Breaking
the agent's dependence on role replication while still leaving a lot of the actual per-resource decision making agent-side.
