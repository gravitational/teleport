# Teleport Enterprise Features

This chapter covers Teleport features that are only available in the commercial
edition of Teleport, called "Teleport Enterprise".

Below is the full list of features that are only available to users of 
Teleport Enterprise:

|Teleport Enterprise Feature|Description
---------|--------------
|[Role Based Access Control (RBAC)](#rbac)|Allows Teleport administrators to define User Roles and restrict each role to specific actions. RBAC also allows to partition cluster nodes into groups with different access permissions.
|[External User Identity Integration](#external-identities)| Allows Teleport to integrate with existing enterprise identity systems. Examples include Active Directory, Github, Google Apps and numerous identity middleware solutions like Auth0, Okta, and so on. Teleport supports LDAP, SAML and OAuth/OpenID Connect protocols to interact with them.
|[Dynamic Configuration](#dynamic-configuration) | Open source edition of Teleport takes its configuration from a single YAML file. Teleport Enterprise can also be controlled at runtime, even programmatically, by dynamiclally updating its configuration.
|[Integration with Kubernetes](#integration-with-kubernetes)| Teleport can be embedded into Kubernetes clusters. This allows Teleport users to deploy and remotely manage Kubernetes on any infrastructure, even behind-firewalls. Teleport embedded into Kubernetes is available as a separate offering called [Telekube](http://gravitational.com/telekube/).
|External Audit Logging | In addition to supporting the local filesystem, Teleport Enterprise is capable of forwarding the audit log to external systems such as Splunk, Alert Logic and others.
|Commercial Support | In addition to these features, Teleport Enterprise also comes with a premium support SLA with guaranteed response times. 

!!! tip "Contact Information":
    If you are interested in Teleport Enterprise or Telekube, please reach out to
    `sales@gravitational.com` for more information.

## RBAC

RBAC stands for `Role Based Access Control`, quoting
[Wikipedia](https://en.wikipedia.org/wiki/Role-based_access_control):

> In computer systems security, role-based access control (RBAC) is an
> approach to restricting system access to authorized users. It is used by the
> majority of enterprises with more than 500 employees, and can implement
> mandatory access control (MAC) or discretionary access control (DAC). RBAC is
> sometimes referred to as role-based security.

Every user in Teleport is **always** assigned a role. OSS Teleport automatically
creates a role-per-user, while Teleport Enterprise allows far greater control over
how roles are created, assigned and managed.

Lets assume your company is using Active Directory to authenticate users, so for a typical 
enterprise deployment you would:

1. Configure Teleport to [use existing user identities](#external-identities) stored 
   in Active Directory.
2. Using Active Directory, assign a user to several groups, perhaps "sales",
   "developers", "admins", "contractors", etc.
3. Create Teleport Roles - perhaps "users", "developers" and "admins".
4. Define mappings from Active Directory groups (claims) to Teleport Roles.

This section covers the process of defining user roles. 

### Roles

A role in Teleport defines the following restrictions for the users who are 
assigned to it:

**OS logins**

The typical OS logins traditionally used. For example, you may not want your interns to login as "root".

**Allowed Labels**

A user will only be granted access to a node if all of the labels defined in
the role are present on the node. This effectively means we use an AND
operator when evaluating access using labels. Two examples of using labels to
restrict access:

1. If you split your infrastructure at a macro level with the labels
`environment: production` and `environment: staging` then you can create roles
that only have access to one environment. Let's say you create an `intern`
role with allow label `environment: staging` then interns will not have access
to production servers.
1. Like above, suppose you split your infrastructure at a macro level with the
labels `environment: production` and `environment: staging`. In addition,
within each environment you want to split the servers used by the frontend and
backend teams, `team: frontend`, `team: backend`. If you have an intern that
joins the frontend team that should only have access to staging, you would
create a role with the following allow labels
`environment: staging, team: frontend`. That would restrict users with the
`intern` role to only staging servers the frontend team uses.

**Session Duration**

Also known as "Session TTL" - a period of time a user is allowed to be logged in.

**Resources**

Resources defines access levels to resources on the backend of Teleport.

Access is either `read` or `write`. Typically you will not set this for users and simply take the default values.
For admins, you often want to give them full read/write access and you can set `resources` to `"*": ["read", "write"]`.

Currently supported resources are:

  * `oidc` - OIDC Connector
  * `cert_authority` - Certificate Authority
  * `tunnel` - Reverse Tunnel (used with trusted clusters)
  * `user` - Teleport users
  * `node` - Teleport nodes
  * `auth_server` - Auth server
  * `proxy` - Proxy server
  * `role` - Teleport roles
  * `namespace` - Teleport namespaces
  * `trusted_cluster` - Trusted Clusters (creates `cert_authority` and `tunnel`).
  * `cluster_auth_preference` - Authentication preferences.
  * `universal_second_factor` - Universal Second Factor (U2F) settings.

**Namespaces**

Namespaces allow you to partition nodes within a single cluster to restrict access to a set of nodes.
To use namespaces, first you need to create a `namespace` resource on the backend then set `namespace`
under `ssh_service` in `teleport.yaml` for each node which you want to be part of said namespace.
For admins, you might want to give them access to all namespaces and you can set `namespaces` to `["*"]`.

The roles are managed as any other resource using [dynamic configuration](#dynamic-configuration) 
commands. For example, let's create a role `intern`.

First, lets define this role using YAML format and save it into `interns-role.yaml`:

```yaml
kind: role
version: v1
metadata:
  description: "This role is for interns"
  name: "intern"
spec:
  # interns can only SSH as 'intern' OS login
  logins: ["intern"]

  # automatically log users out after 8 hours
  max_session_ttl: 8h0m0s

  # Interns will only be allowed to SSH into machines 
  # with the label 'environment' set to 'staging'
  node_labels:
    "environment": "staging"
```

Now, we just have to create this role:

```bash
$ tctl create -f interns-role.yaml
```

## External Identities

The standard OSS edition of Teleport stores user accounts using a local storage
back-end, typically on a file system or using a highly available database like `etcd`. 

Teleport Enterprise allows the administrators to integrate Teleport clusters
with existing user identities like Active Directory or Google Apps using protocols
like LDAP, OpenID/OAuth2 or SAML. Refer to the following links for additional
additional integration documentation:

* [OpenID Connect (OIDC)](oidc.md)
* [Security Assertion Markup Language 2.0 (SAML 2.0)](saml.md)

In addition, Teleport Enterprise can query for users' group membership and assign different
roles to different groups, see the [RBAC section](#rbac) for more details.

## Dynamic Configuration

OSS Teleport reads its configuration from a single YAML file,
usually located in `/etc/teleport.yaml`. Teleport Enterprise extends that by
allowing cluster administrators to dynamically update certain configuration
parameters while Teleport is running. This can also be done programmatically.

Teleport treats such dynamic settings as objects, also called "resources".
Each resource can be described in a YAML format and can be created, updated or
deleted at runtime through three `tctl` commands:

| Command Example | Description
|---------|------------------------------------------------------------------------
| `tctl create -f tc.yaml`  | Creates the trusted cluster described in `tc.yaml` resource file.
| `tctl del -f tc.yaml`     | Deletes the trusted cluster described in `tc.yaml` resource file.
| `tctl update -f tc.yaml`  | Updates the trusted cluster described in `tc.yaml` resource file.

This is very similar how the `kubectl` command works in
[Kubernetes](https://en.wikipedia.org/wiki/Kubernetes).

Two resources are supported currently:

* See [Trusted Clusters](#dynamic-trusted-clusters): to dynamically connect / disconnect remote Teleport clusters.
* See [User Roles](#rbac): to create or update user permissions on the fly.

### Dynamic Trusted Clusters

See the [Dynamic Trusted Clusters](trustedclusters.md) more more details and examples.

### Authentication Preferences

Using dynamic configuration you can also view and change the type of cluster authentication Teleport supports at runtime.

#### Viewing Authentication Preferences

You can query the Cluster Authentication Preferences (abbreviated `cap`) resource using `tctl` to find out what your current authentication preferences are.

```
$ tctl get cap
Type      Second Factor
----      -------------
local     u2f
```

In the above example we are using local accounts and the Second Factor used is Universal Second Factor (U2F, abbreviated `u2f`), once again to drill down and get more details you can use `tctl`:

```
$ tctl get u2f
App ID                     Facets
------                     ------
https://localhost:3080     ["https://localhost" "https://localhost:3080"]
```

#### Updating Authentication Preferences

To update Cluster Authentication Preferences, you'll need to update the resources you viewed before. You can do that creating the following file on disk and then update the backend with `tctl create -f {filename}`.

```yaml
kind: cluster_auth_preference
version: v2
metadata:
  description: ""
  name: "cluster-auth-preference"
  namespace: "default"
spec:
  type: local         # allowable types are local or oidc
  second_factor: otp  # allowable second factors are none, otp, or u2f.
```

If your Second Factor Authentication type is U2F, you'll need to create an additional resource:


```yaml
kind: universal_second_factor
version: v2
metadata:
  description: ""
  name: "universal-second-factor"
  namespace: "default"
spec:
  app_id: "https://localhost:3080"
  facets: ["https://localhost", "https://localhost:3080"]
```

If you are not using local accounts but rather an external identity provider like OIDC, you'll need to create an OIDC resource like below.

```yaml
kind: oidc
version: v2
metadata:
  description: ""
  name: "example"
  namespace: "default"
spec:
  issuer_url: https://accounts.example.com
  client_id: 00000000000000000.example.com
  client_secret: 00000000-0000-0000-0000-000000000000
  redirect_url: https://localhost:3080/v1/webapi/oidc/callback
  display: "Welcome to Example.com"
  scope: ["email"]
  claims_to_roles: 
    - {claim: "email", value: "foo@example.com", roles: ["admin"]}
```

## Integration With Kubernetes

Gravitational maintains a [Kubernetes](https://kubernetes.io/) distribution
with Teleport Enterprise integrated, called [Telekube](http://gravitational.com/telekube/). 

Telekube's aim is to dramatically lower the cost of Kubernetes management in a
multi-region / multi-site environment. 

Its highlights:

* Quickly create Kubernetes clusters on any infrastructure.
* Every cluster includes an SSH bastion and can be managed remotely even if behind a firewall.
* Every Kubernetes cluster becomes a Teleport cluster, with all Teleport
  capabilities like session recording, audit, etc.
* Every cluster is dependency free and automomous, i.e. highly available (HA) 
  and includes a built-in caching Docker registry.
* Automated remote cluster upgrades.

Typical users of Telekube are:

* Software companies who want to deploy their Kubernetes applications into
  the infrastructure owned by their customers, i.e. "on-premise".
* Managed Service Providers (MSPs) who manage Kubernetes clusters for their
  clients.
* Enterprises who run many Kubernetes clusters in multiple geographically 
  distributed regions / clouds.

!!! tip "Contact Information":
    For more information about Telekube please reach out us to `sales@gravitational.com` or fill out the contact for on our [website](http://gravitational.com/)
