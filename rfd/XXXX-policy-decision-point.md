# Policy Decision Point

This proposal describes a means of reworking teleport access-control decisions to remove
the need to perform access-control decisions on peripheral agents and to bring teleport's
internals more inline with common access-control best practices.


## Problem

Access-control decisions in teleport are typically made at the agent, and often times a single operation
will incur many small access-control decisions over the course of its execution. This distributed and
innervated model of access-control decision making has some benefits.  For example, early versions of
teleport made it a priority to have agents continue to be able to make access-control decisions even
when the auth service was unavailable for long periods of time. However, there are also serious
drawbacks to the distributed and innervated model:

- Scalability: Access-control decisions at the agent require that agents have access to all necessary
information in order to be able to make correct access-control decisions. This forces us to encode a
large amout of otherwise unnecessary information into certificates, and forces agents to cache all
resources relevant to access-control decision making (namely roles).  In some larger clusters, role
replication alone can account for a significant portion of all load incurred after an interruption
of auth connectivity.

- Compatibility: Because teleport access-control includes the ability to specify unscoped deny rules,
there is no sane way to represent many new access-control features to older agents s.t. they can continue
to make sensible decisions. We've typically had to resort to combersome and imperfact hacks that negatively
impact user experience, such as injecting wildcard deny rules into the caches of outdated agents to force
them to deny any access that might require knowledge of a new feature.

- Maintainability: From both the developer and user perspective, distributed and innervated access-control
decision making poses maintainability challenges. For developers, it is difficult to reason about or change
acccess-control features because the implementation is so spread out, and so many different teleport
components rely on internal details of the implementation. For users, in order to take advantage of a bug
fix or new feature, they must often update thousands of teleport installations. This is particularly
worrysome in the case of security fixes, as access-control decision making is a particularly sensitive
part of teleport.

- Development Cadence: Due mostly to the aforementiond issues, it is very difficult to make any non-trivial
changes to teleport access-control decision making. Most major proposed changes to teleport access-control
decision making (e.g. scoped rbac) have never even reached the point of a working prototype. This isn't
only because of the lack of a clear abstraction boundary, but that is a very significant contributing factor.


## Proposal

### Key Terminology

- **Policy Decision Point (PDP)**: The component responsible for making access-control decisions. 

- **Policy Enforcement Point (PEP)**: The component responsible for enforcing access-control decisions.


#### Overview

We will rework teleport's access-control decision making to have a well-defined boundary between
Policy Decision Point and Policy Enforcement Point. The goal of this rework will be to abstract
as much complexity as possible into the PDP, while attempting to minimize the need to actually
change how or where enforcement is done today. The PDP/PEP boundary will be representable as a
GRPC API, but where possible we will continue to ensure that teleport does not make unnecessary
network round trips.

We will also establish a set of conventions intended to make the transition to using the new
system as easy as possible, and to ensure that it is as difficult as possible to accidentally
misuse it either by misunderstanding or oversight.


### API

The PDP/PEP boundary will be represented as a GRPC API, with methods for the different kinds of decisions.
Because teleport as a whole is an integrated PDP/PEP, our decisions are often much more nuanced than a
simple allow/deny. This is one of teleport's greatest strenghts, as this allows us to provide very granular
controls over actions. It also means that many of the decisions we make are very verbose. Take ssh access
for example: an 'allow' decision for ssh access must include parameters for agent forwarding, port forwaring,
concurrent connection limits, bpf events, expired cert disconnenct, locking, file copying, etc. For this reason
we will eschew any attempt at creating a "unified" method for decisisons in favor of implementing custom
methods and types intended to meet the specific needs of specific decisions.

In order to ensure that the PDP API remains simple and usabe we will establish a set of convetions for how
decision methods should be handled.

- Methods will take the form `rpc Evaluate<Decision>(Evaluate<Decision>Request) returns (Evaluate<Decision>Response)`.

- Request messages will contain a common metadata field, plus any fields necessary to describe the decision.
At least the teleport username and any necessary identifiers for the object of access.

- Response messages will be a single onof with an allow and a deny types of the form `<Decision>Permit`
and `<Decision>Denial` respectively.

- Denial messages will contain at least a sanitized user-facing message, plus any additional information that may be useful for debugging/auditing, and or to inform retry behavior if appropriate.

- Permit messages will encode all necessary parameters for the PEP to properly enforce the allow decision, and
will be the primary means by which peripheral agents model permissions. Any agent-side logic that currently
relies on some combination of certificate identity and role set will be reworked to require the appropriate
Permit message as one of its inputs. Permit messages will always be passed by reference, not by value, to
ensure that zero value bugs "fail safe" via panic.

Here is a truncated example of the PDP API for server access:

```grpc
service PolicyDecisionPoint {
  
  rpc EvaluateServerAccess(EvaluateServerAccessRequest) returns (EvaluateServerAccessResponse);

  // ...
}

message EvaluateServerAccessRequest {
  EvaluationMetadata metadata = 1;  
 
  string teleport_user = 2;
 
  string server_id = 3;
 
  string login = 4;
}

message EvaluateServerAccessResponse {
  oneof result {
      ServerAccessPermit allow = 1;
      ServerAccessDenial deny = 2;
  }
}

message ServerAccessPermit {
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

message ServerAccessDenial {
  DenialMetadata metadata = 1;

  // ...
}
```

With the above API, most server access enforcement logic can be ported to the PDP with minimal changes.
`AccessChecker.EnhancedRecordingSet()` becomes `permit.Bpf`, `AccessChecker.GetHostSudoerts(server)` becomes
`permit.HostSudoers`, etc.


### Phased Rollout

The switch to PDP will be accomplished in two distinct phases. An "refactor" phase, and a "relocate" phase.

#### Refactor Phase

The refactor phase will consist of initial implementation of the PDP API, without any outward changes that might
affect cross-version compatibility. In order to seamlessly transition from decisions being made at agents to decisions
being made at control plane, the implementation of the PDP logic will be polymorphic over two possible operating modes
depending on where it is running.

On the control plane, the PDP will have access to the entire set of teleport users and roles. This variant
will be able to serve as the backing of the GRPC API, and make internal decisions as if it were a remote PDP.
This PDP will be a shared service initialized once at process startup.

On agents, a local ephemeral PDP will be able to be initialized with a fake user store providing only the identity
of the user making the request. This variant of the PDP will need to be initialized for each incoming request, and
will be able to only serve requests related to the user for which it was initialized (much like the AccessChecker
today).

Common PDP logic will be shared between the two variants, meaning that so long as we need to support the agent-local
PDP, the control-plane PDP won't be able to make use of any state that cannot be derived from a combination of local
agent cache and user certificate identity.

The intent of this work is to allow us to make the vast majority of necessary code changes in a totally backwards-compatible
manner and to backport them to all active release branches. This will ensure that ongoing work related to policy enforcement
doesn't need to reimplement the same logic for the old model when backporting.

During this phase we will also implement `tctl` commands for directly invoking the PDP GRCP API on the control plane. This
will provide a powerful debugging and auditing tool for superusers and devlopers who want deeper insight into how teleport
makes decisions.


#### Relocate Phase

The relocate phase will see us actually transition to all access-control decisions being made at the control plane.
The method of relocation well depend on the kind of teleport agent. For tunnel agents, we will start performing denial
at the control plane itself, and allowed dials will have the Grant object forwarded to the agent as part of the dial.
For direct dial agents, the agent will call-back into the control plane to get a decision.


TODO
