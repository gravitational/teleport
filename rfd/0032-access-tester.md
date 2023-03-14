---
authors: Rui Li (rui@goteleport.com)
state: deprecated
---

# RFD 32 - Datalog based access tester

**Deprecated:** `roletester` has been removed for the Teleport 10 release.

## What
This document will describe the implementation for a Datalog based access tester for Teleport's Role-Based Access Control system that will be able to answer access-related questions.

The access tester should allow admins to answer questions such as:
- Can user Jean SSH into node-1 as root?
- Which roles are preventing user Jean access to node-1?
- Which nodes can user Jean access?
- Which nodes have Jean been denied access to?

## Why
Teleport provides a Role-Based Access Control (RBAC) that allows admins to allow or deny access to specific cluster resources based on defined labels.

Role configurations can be complex and Teleport currently does not provide good tools for admins to troubleshoot any access related issues (e.g. why can't this user login to this node?).

## Scope
This document will only be focusing on determining access to SSH nodes. The proposed access tester can be easily extended to determine access to database, application, kubernetes etc. By providing separate queries for different access types, we can also answer more specific queries (e.g. which databases does this user have access to? Which roles allow access to database production as user postgres?).

## Architecture
For simple use cases and better UX, a provided tctl command can be used to query for access-related questions. For advanced use-cases, a provided admin tool lets users execute Datalog queries directly using a command or in a REPL-like shell.

There are a few options for how we can architecture this access tester:
- Real-time deductive database (Datomic)
- Extend existing Go Datalog libraries to use negation and build access tester directly into Teleport
- Write own Go Datalog library and build access tester directly into Teleport
- Use Rust library (Crepe) and connect to Teleport via Rust grpc client, and would act as a standalone program
- Use Rust library (Crepe) and call Rust from Go to integrate directly with tctl

### Using Rust

**Pros:**
- Using high quality datalog library (crepe)
  - In-memory, so is probably faster than the database option
- Introducing Rust into codebase opens doors for us to start using more Rust
- POC already built using rust library

**Cons:**
- Integration with tctl is a bit more involved
  - Will have to look into calling Rust from Go

### Deductive database
```
                |‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾|        |‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾‾|
teleport <----> | access tester | <----> | realtime deductive db |
                |_______________|        |_______________________|
```
**Pros:**
- Simple integration with REST API (if available)
- Supports many features

**Cons:**
- Client API is in Clojure
- Overkill for our use case
  - We don't need persistence since we already have all the data in Teleport
- Non-free for Datomic
- Extra database node running for our clients

### Extend existing Go Datalog libraries

**Pros:**
- Integration with tctl is simple
- Faster to implement than building own library (depending on quality)
- In-memory, so could be faster than database depending on implementation

**Cons:**
- If library quality is bad, could slow down development considerably
- Not many good Go datalog libraries with the appropriate licenses

### Build our own Datalog library

**Pros:**
- Integration with tctl is simple
- In-memory, so could be faster than database depending on implementation

**Cons:**
- Hard to gauge how difficult implementation is

Ultimately, going for the Rust and Go interop solution is the best option. This not only introduces Rust into the code base, but provides a way to utilize Rust Datalog libraries with Rust and Go FFI. Using protobufs, we would be able to painlessly pass inputs and outputs between the two languages.

## Details

### Context
Datalog is a logic programming language that is a subset of Prolog. A common use case for Datalog is as a query language for deductive databases, which is exactly what we need in order to answer access-related questions. Datalog programs are written as a collection of logical constraints which the compiler uses to compute the solution (using whatever method/algorithm it pleases). These logical constraints are sentences in logical form and are a finite set of **facts**, which are assertions of the world in which we operate; and **rules**, which are sentences that allow us to infer new facts from existing ones. This document will describe a model for representing Teleport's RBAC system as a logical model which we can then query to answer our desired questions.

### Facts
Facts (or predicates) represent assertions of the world in which we operate.For translating Teleport's RBAC system, we can take everything we know from the role configuration and define what we need.

We will need to determine the labels on the node itself, the role's defined allow/deny labels, the role's define logins, and the user to role facts.

Fact | Example | Interpretation
--- | --- | ---
`HasRole(user, role)` | `HasRole(jean, dev)` | User 'jean' has role 'dev'
`HasTrait(user, trait_key, trait_value)` | `HasTrait(jean, login, dev)` | User 'jean' has the login trait 'dev'
`NodeHasLabel(node, label_key, label_value)` | `NodeHasLabel(node-1, environment, staging)` | SSH node 'node-1' has the label 'environment:staging'
`RoleAllowsNodeLabel(role, label_key, label_value)` | `RoleAllowsNodeLabel(dev, environment, staging)` | Role 'dev' is allowed access to SSH nodes with label 'environment:staging'
`RoleDeniesNodeLabel(role, label_key, label_value)` | `RoleDeniesNodeLabel(bad, environment, production)` | Role 'bad' is denied access to SSH nodes with label 'environment:production'
`RoleAllowsLogin(role, login)` | `RoleAllowsLogin(admin, root)` | Role 'admin' can login as os user 'root' to SSH nodes
`RoleDeniesLogin(role, login)` | `RoleDeniesLogin(dev, root)` | Role 'dev' cannot login as os user 'root' to SSH nodes

These facts are then used within rules to answer questions. The facts themselves can also be queried, for example HasRole(jean, Role)? will return all the roles that user 'jean' has where the term 'Role' represents an arbitrary variable and 'jean' is a constant.

One concern would be that a user can have multiple roles. The way Datalog works allows for this one to many mapping by just adding multiple facts, which in this case is multiple HasRole facts for the user. For example:

```prolog
HasRole(jean, dev).
HasRole(jean, cloud).
HasRole(jean, admin).
```
means that Jean has roles 'dev', 'cloud', and 'admin'. Similarly, this representation also applies to labels and logins for roles and nodes.

### Rules
Rules are sentences that allow us to infer new facts from existing ones. We can combine multiple rules to create new rules. The most important question for the access tester is whether a user can access a given node as an os user, and we will define this as HasAccess(User, Login, Node, Role). Other rules will provide other contextually related facts that will help answer this overarching question. I have grouped the rules based on what their queries would mean so it is more clear what each rule is defining.

***Does the given role allow or deny access to a node?***
Rule | Logical interpretation
--- | ---
`HasAllowNodeLabel(Role, Node, Key, Value) :- RoleAllowsNodeLabel(Role, Key, Value), NodeHasLabel(Node, Key, Value)` | If the role has allow labels and the node has the same labels, then the role allows access to the node
`HasDenyNodeLabel(Role, Node, Key, Value) :- RoleDeniesNodeLabel(Role, Key, Value), NodeHasLabel(Node, Key, Value)` | If the role has deny labels and the node has the same labels, then the role denies access to the node

***Does the given user have access to a node as an os user?***
Rule | Logical interpretation
--- | ---
`HasAllowRole(User, Login, Node, Role) :- HasRole(User, Role), HasAllowNodeLabel(Role, Node, Key, Value), RoleAllowsLogin(Role, Login), not(RoleDeniesLogin(Role, Login)), not(HasLoginTrait(User))` | If the user has a role, the role allows access to the node, the role has a login that is not denied and the user does not have a defined login trait, then the user has a role that gives them access to the node as login
`HasAllowRole(User, Login, Node, Role) :- HasRole(User, Role), HasAllowNodeLabel(Role, Node, Key, Value), HasTrait(User, login, Login), not(RoleDeniesLogin(Role, Login)), not(RoleDeniesLogin(Role, login))` | If the user has a role, the role allows access to the node and the user has a defined login trait that is not denied, then the user has a role that gives them access to the node as login
`HasDenyRole(User, Node, Role) :- HasRole(User, Role), HasDenyNodeLabel(Role, Node, Key, Value)` | If the user has a role and the role denies access to the node, then the user has a role that denies them access to the node
`HasDeniedLogin(User, Login, Role) :- HasRole(User, Role), RoleDeniesLogin(Role, Login)` | If the user has a role, and the role denies a login, then the user has a role that denies them access with the specified login
`HasDeniedLogin(User, Login, Role) :- HasRole(User, Role), HasTrait(User, login, Login), RoleDeniesLogin(Role, login)` | If the user has a role and login trait, and the role denies the login traits, then the user has a role that denies them access with the all login traits
`HasAccess(User, Login, Node, Role) :- HasAllowRole(User, Login, Node, Role), not(HasDenyRole(User, Node, Role)), not(HasDeniedLogin(User, Login, Role))` | If the user has a role that gives them access to the node as login and there are no roles denying access to the node as login, then the user has access to the node as login

***Which roles allow or deny a user access to a given node?***
Rule | Logical interpretation
--- | ---
`DenyAccess(User, Login, Node, Role) :- HasDenyRole(User, Node, Role), HasTrait(User, login, Login)` | If the user has a role and login trait and the role denies access to the node, then that specific role denies the user access to node as the login trait
`DenyAccess(User, Login, Node, Role) :- HasDenyRole(User, Node, Role), HasAllowRole(User, Login, Node, OtherRole)` | If the user has a role that denies them access, but also has other roles allowing access, then that specific role denies the user access to the node
`DenyLogins(User, Login, Role) :- HasDeniedLogin(User, Login, Role)` | If the user is denied a login by role, then that login is denied to the user for all nodes

Rules can be thought of as a simple implication between the body (the part after :-) and the head (the part before :-) where if body is true, then it implies the head.

### Examples
Some common questions are listed below. It is important to note that if we provide variables instead of constants to the queries, we will get a list of term values for the variable based on the facts that exist or are inferred. For example, HasAccess(jean, root, Node)? will return all the SSH nodes that user 'jean' has access to as 'root'. Similarly, the queries AllowRoles? and DenyRoles? can be used in this way to get the corresponding roles that deny/allow a user access to a specific SSH node.

Example | Query interpretation
--- | ---
`HasAllowNodeLabel(dev, node-1, environment, staging)?` | Does the role 'dev' allow access to SSH node 'node-1' given the label 'environment:staging'?
`HasDenyNodeLabel(bad, node-1, environment, production)?` | Does the role 'bad' deny access to SSH node 'node-1' given the label 'environment:production'?
`HasAllowRole(jean, root, node-1, Role)?` | Does the user 'jean' have a role that gives them access to SSH node 'node-1' as 'root'?
`HasDenyRole(jean, node-1, Role)?` | Does the user 'jean' have a role that denies them access to SSH node 'node-1'?
`HasAccess(jean, root, node-1, Role)?` | Does user 'jean' have access to SSH node 'node-1' as 'root'?
`HasAccess(jean, root, Node, Role)?` | Which SSH nodes does user 'jean' have access to as 'root'?
`HasAccess(jean, Login, Node, Role)?` | Which SSH nodes does user 'jean' have access to?
`DenyAccess(jean, Login, node-1, Role)?` | Which roles deny user 'jean' to access SSH node 'node-1'?
`DenyAccess(jean, Login, Node, Role)?` | Which nodes are user 'jean' denied access to? (will be returned as Role/Node/Login pairs)
`DenyLogins(jean, Login, Role)?` | Which logins are user 'jean' denied access to on all nodes?
