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

RBAC stands for `Role Based Access Control`, quoting [Wikipedia](https://en.wikipedia.org/wiki/Role-based_access_control):

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

### Teleport Role

A role in Teleport defines the following restrictions for the users who are 
assigned to it:

* **OS logins**: The typical OS logins traditionally used. For example, you may not want your interns
  to login as "root".
* **Allowed Labels**: A user will be allowed to only log in to the
  nodes with these labels. Perhaps you want to label your staging nodes 
  with the "staging" label and update the `intern` role such that the interns
  won't be able to SSH into a production machine by accident.
* **Session Duration**: Also known as "Session TTL" - a period of time a user
  is allowed to be logged in.

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
like LDAP, OpenID/OAuth2 or SAML.

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

The section below will re-create the example configuration
from the [Trusted Clusters section](admin-guide.md#trusted-clusters) in the admin manual using dynamic resources.
If you have not already, it will be helpful to review this section first.

To add behind-the-firewall machines and restrict access so only users with role
"admin" can access them, do the following:

First, create a static or dynamic token on `main` that will be used by `cluster-b`
to join `main`. A dynamic token can be generated by running:
`tctl nodes add --ttl=5m --roles=trustedcluster` and a static token can be
generated out-of-band and added to your configuration file like so:

```yaml
auth_service:
  enabled: yes
  cluster_name: main
  tokens:
    # generate a large random number for your token, we recommend
    # using a tool like `pwgen` to generate sufficiently random
    # tokens of length greater than 32 bytes
    - "trustedcluster:fake-token"
```

Then, create a `TrustedCluster` resource on `cluster-b` that tells `cluster-b`
how to connect to main and the token created in the previous step for
authorization and authentication. To do that, copy the resource below
to a file called `trusted-cluster.yaml`.

```yaml
kind: trusted_cluster
version: v1
metadata:
  description: "Trusted Cluster B"
  name: "Cluster B"
  namespace: "default"
spec:
  enabled: true
  roles: [ "admin" ]
  token: "fake-token"
  tunnel_addr: <main-addr>:3024
  web_proxy_addr: <main-addr>:3080
```

Notice how we defined `roles` in the `TrustedCluster` resource. This is
the role assumed by any user when they connect from `main` to `cluster-b`.
That means we need to make sure the `admin` role exists in Teleport and we
need it associate it with a user (let's say the user is named "john"). To do
that, copy the resource below to a file called`admin-role.yaml`.

```yaml
kind: role
version: v1
metadata:
  description: "Admin Role"
  name: "admin"
spec:
  logins: [ "john", "root" ]
  max_session_ttl: 90h0m0s
```

Now inject both roles into the Teleport "auth service" on `cluster-b` using `tctl`:

```bash
$ tctl create -f  trusted-cluster.yaml
$ tctl create -f  admin-role.yaml
```

That's it. To verify that the trusted cluster is online:

```bash
$ tsh --proxy=main.proxy clusters
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
