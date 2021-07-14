---
authors: Roman Tkachenko (roman@goteleport.com)
state: draft
---

# RFD 31 - Dynamic Registration for Apps and Databases

## What

Provide ability for users to register/deregister web apps (for App Access)
and databases (for Database Access) without having to modify static yaml
configuration or spin up/bring down app/database agents.

## Why

Currently, users have two options for adding/removing web apps and databases
in a Teleport cluster:

- Update yaml configuration of a particular `app_service` or `db_service`
  to add/remove an app or a database entry and restart the service.
- Bring up a new instance of an `app_service` or a `db_service` agent to
  register a new app/database, or stop an existing one to deregister.

This is inconvenient for multiple reasons:

- Updating static yaml configuration and restarting the agent is manual and
  affects all other services served by this agent (incl. other apps/databases).
- Bringing up or stopping an app/database service agent can be challenging
  from operational perspective.
- It is unfriendly to automation and from the integration standpoint i.e. no
  easy way for a CI tool or an external plugin to connect a new app/database.
- Similar to above, it makes it impossible for tools like our Terraform
  provider to manage apps/databases as resources.

## Scope

Given the shortcomings of the current approach, this RFD proposes the way to:

- Allow Teleport users with appropriate permissions to add/remove web apps and
  databases using CLI and API.

The following is **out of scope** of this RFD and/or initial implementation:

- Providing ability to add/remove web apps and databases via Teleport web UI.
- Auto-discovering app and database endpoints in Kubernetes clusters is kept
  out of initial implementation scope but we do discuss future work options
  (see relevant [Github issue](https://github.com/gravitational/teleport/issues/4705)
  and [Kubernetes](#kubernetes) section).

## How

The basic design premise is to employ familiar to Teleport users resource-based
approach:

- Web apps and databases are represented as yaml resources that can be managed
  using `tctl create/get/rm` commands.
- App/database service agents watch their respective resources and update
  their proxied app/database configurations appropriately.
- We will use labels to instruct app/database service agents to watch for
  specific app/database resources.

## UX

Before diving into implementation details, let's explore a few scenarios of
how different UX personas may use this feature. For all scenarios we are
assuming that Teleport cluster has an app/database service agent running and
configured to watch for appropriate resources.

**Cluster admin**

Teleport cluster admin will use `tctl` resource commands to create a yaml
resource representing a web app or a database, or remove it.

```bash
$ tctl create grafana.yaml
$ tctl rm db_servers/aurora
```

When the resource is created, an appropriate app/database service agent will
pick it up and start serving.

In future, it might make sense for us to expose this functionality in web UI,
in which case interactive Teleport users with appropriate permissions will be
able to do it as well.

**CI system**

An external system or a user can run `tctl` commands using identity files and
otherwise is no different than using `tctl` locally on the auth server.

```bash
$ tctl auth sign --ttl=24h --user=drone --out=drone.pem
$ tctl --auth-server=192.168.0.1:3025 -i drone.pem create grafana.yaml
```

**API user**

Teleport [API client](https://github.com/gravitational/teleport/blob/master/api/client/client.go)
will provide methods for integrators to create/update/delete web apps and databases.
Once created or deleted, the change will be picked up by an appropriate agent.

See [API changes](#api-changes) section below for more details.

**Terraform provider**

Terraform provider uses API client for resource management so it will use the
client's get, create and delete methods as any other API user.

This, again, assumes that there is a app/database service in the cluster that
adds/removes web apps and databases.

**Cloud user**

Cloud users should be able to use `tctl` commands with [impersonation](https://goteleport.com/docs/ver/6.2/access-controls/guides/impersonation/)
which is similar to the CI/external user scenario.

## Managing app/database resources

The specs of app/database resources closely resemble their respective yaml
configuration sections.

Applications are currently a part of the [Server](https://github.com/gravitational/teleport/blob/v7.0.0-dev.9/api/types/types.proto#L232)
which is actually only supposed to represent SSH nodes. Hence, we need to
factor apps out into own `AppServer` resource with the following spec:

```yaml
# grafana-app.yaml
kind: app_server # See impl. note below for kind explanation.
version: v3
metadata:
  name: grafana
  labels:
    env: dev
spec:
  description: Grafana
  uri: http://localhost:3000
  public_addr: grafana.example.com
  rewrite:
    headers:
    - name: X-Custom-Trait-Env
      value: "{{external.env}}"
  dynamic_labels:
    date:
      command: ["/bin/date"]
      period: 1m
```

[DatabaseServer](https://github.com/gravitational/teleport/blob/v7.0.0-dev.9/api/types/databaseserver.go)
is already a separate resource which we should be able to use as-is:

```yaml
# redshift-db.yaml
kind: db_server # See impl. note below for kind explanation.
version: v3
metadata:
  name: redshift
  labels:
    env: aws
spec:
  description: Amazon Redshift
  protocol: postgres
  uri: redshift-cluster-1.abcdefg.us-east-1.redshift.amazonaws.com:5439
  ca_cert: file:// or pem data
  aws:
    region: us-east-1
    redshift:
      cluster_id: redshift-cluster-1
  dynamic_labels:
    date:
      command: ["/bin/date"]
      period: 1m
```

Users can use `tctl` commands to manage these just like any other resource:

```bash
$ tctl create grafana-app.yaml
$ tctl create redshift-db.yaml
$ tctl get app_servers
$ tctl get db_servers
$ tctl get app_servers/grafana
$ tctl rm db_servers/redshift
```

**Refactoring & implementation notes**

- Apps are currently a part of the [Server](https://github.com/gravitational/teleport/blob/v7.0.0-dev.9/api/types/types.proto#L232)
  object. As a part of this work, refactor it into a separate `AppServer` object
  similar to [DatabaseServer](https://github.com/gravitational/teleport/blob/v7.0.0-dev.9/api/types/databaseserver.go).
- Rename apps `labels` and `commands` to `static_labels` and `dynamic_labels`
  respectively, internally and in file configuration, in backwards compatible
  fashion.
- App/database resources currently have kinds `app_server` and `db_server`
  respectively. As a part of this cleanup, we can alias them to `app` and `db`
  which would be cleaner (but require some extra work).

## Configuring app/database service

In addition to apps/databases from static yaml configuration, app/database
services can be configured to watch for the above resources with specific
labels. If no selectors are specified, no resources are being watched.

```yaml
app_service:
  enabled: "yes"
  # Watch for apps with app=ops&env=dev or env=test labels.
  selectors:
  - matchLabels:
      app: ops
      env: dev
  - matchLabels:
      env: test
  # Existing static apps configuration, may be omitted.
  apps:
    ...
```

```yaml
db_service:
  enabled: "yes"
  # Watch for any database with env label set.
  selector:
  - matchLabels:
      env: "*"
  # Existing static databases configuration, may be omitted.
  dbs:
    ...
```

The `matchLabels` approach is inspired by Kubernetes selectors. It is the only
type of selector supported initially, but gives us a way to introduce other
types in future if necessary (e.g. K8s has `matchExpression`).

To make a service watch for any app/database, an explicit wildcard selector
must be specified:

```yaml
selector:
- matchLabels:
    "*": "*"
```

**Static vs dynamic configuration**

There must be a way to distinguish between apps and databases from the static
yaml configuration vs ones registered as dynamic resources. This is needed,
for example, to prevent users from deleting those registered statically.

We will use the same approach as in [RFD16 Dynamic Configuration](./0016-dynamic-configuration.md)
and use `teleport.dev/origin` label to denote whether the app/database resource
comes from static configuration or created dynamically. The rules are:

- Users can't delete apps/databases with `teleport.dev/origin: config-file`
  label.
- Users can't set `teleport.dev/origin` label in their resources - it will be
  auto-set to `dynamic` when they're created.
- Users can't create apps/databases with the same name as one of existing
  static apps/databases (to avoid various conflict resolution issues).

**Multiple replicas**

When multiple app/database agents pick up the same web app or database resource,
they all add it to their proxying configuration. The connections that go to
this app/database follow the same load-balancing rules which already exist
today. This is no different than registering the same app/database multiple
times via static configuration today.

## Security implications

In the current Teleport security model, the trust is established between
components of the system i.e. when a new app or database service agent joins
the cluster, it needs to present a valid auth token. Once the service has
registered, updating its configuration to connect new web apps or databases
does not require re-registering the service with the cluster.

Dynamic app/database registration plugs into this model - when a new app or
database resource is created, it is picked up by a service that is already
part of the cluster. Managing app/database resources themselves is subject to
RBAC checks so only users with appropriate permissions to `app_server` and
`db_server` resources are allowed to create/remove them.

## API changes

Teleport API client currently provides the following methods for managing
app/database resources (copied from [api/client/client.go](https://github.com/gravitational/teleport/blob/v7.0.0-dev.9/api/client/client.go)):

```go
GetAppServers(ctx context.Context, namespace string, skipValidation bool) ([]types.Server, error)
UpsertAppServer(ctx context.Context, server types.Server) (*types.KeepAlive, error)
DeleteAppServer(ctx context.Context, namespace, name string) error
```

```go
GetDatabaseServers(ctx context.Context, namespace string, skipValidation bool) ([]types.DatabaseServer, error)
UpsertDatabaseServer(ctx context.Context, server types.DatabaseServer) (*types.KeepAlive, error)
DeleteDatabaseServer(ctx context.Context, namespace, hostID, name string) error
```

These methods were designed for internal usage by Teleport components e.g. when
app/database agent registers with the cluster or for RBAC checks. As such, they
are quite awkward for API client users. Changes to make them more API friendly:

* If we decide to settle on `app` and `db` terminology (vs current `app_server`
  and `db_server`), name them appropriately like `GetApps` or `DeleteDatabase`.
* Switch to `Create` and `Update` methods instead of `Upsert` anti-pattern.
* Remove extra arguments that either aren't used or don't make sense for users
  such as `namespace` (always "default"), `skipValidation`, `hostID`, etc.
* Add methods for querying individual apps/databases by names which is useful
  for Terraform provider.

Cleaned up version of the apps API (using Go syntax instead of `.proto` for
brevity):

```go
GetApps(ctx context.Context) ([]types.AppServer, error)
GetApp(ctx context.Context, name string) (types.AppServer, error)
CreateApp(ctx context.Context, app types.AppServer) error
UpdateApp(ctx context.Context, app types.AppServer) error
DeleteApp(ctx context.Context, name string) error
```

```go
GetDatabases(ctx context.Context) ([]types.DatabaseServer, error)
GetDatabase(ctx context.Context, name string) (types.DatabaseServer, error)
CreateDatabase(ctx context.Context, db types.DatabaseServer) error
UpdateDatabase(ctx context.Context, db types.DatabaseServer) error
DeleteDatabase(ctx context.Context, name string) error
```

## Kubernetes

Discovering web app and database endpoints in Kubernetes clusters is kept out of
the initial implementation scope. Initially, users can use CLI or API approach
described in this RFD to add or remove web app and database Teleport resources,
which works for clusters deployed both on-prem and in Kubernetes.

There are options though to make the process of adding new apps and databases
friendlier and more "idiomatic" for Kubernetes users, by implementing a
controller that will be monitoring specific K8s resources.

The simpler option is to monitor `Service` objects with specific labels or
annotations. It may work for very simple use-cases but the downside is that
`Service` object can't fully express Teleport application (e.g. public address,
rewrite configuration, etc.) or a database endpoint (e.g. protocol, cloud
specific settings, etc.). Unless maybe we use annotations to encode this
information which isn't ideal.

A better approach is to use Kubernetes CRDs and define custom resources e.g.
`teleport.sh/app` and `teleport.sh/db` which provides versioning and ability to
define all custom fields these resources need. These resources are registered
when users install our Helm chart.

When run inside Kubernetes, app and database service agents will start a K8s
controller which will be watching these custom resources using `selector`
described above to filter them by labels, so no new configuration is necessary.
