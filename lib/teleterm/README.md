# lib/teleterm

`lib/teleterm` contains code used by the tsh daemon. The tsh daemon serves as a backend service for
[Teleport Connect](https://goteleport.com/connect/).

The frontend part of Teleport Connect lives in
[`webapps/packages/teleterm`](https://github.com/gravitational/webapps/tree/master/packages/teleterm).

This README serves strictly as an overview of the code in `lib/teleterm` only. See [RFD
63](/rfd/0063-teleport-terminal.md) for the general overview of the architecture behind Teleport
Connect.

## Startup

During startup, the Electron app executes `tsh daemon start` in a separate process. This creates a
long-lived instance of `daemon.Service` and starts the gRPC server responsible for communication
between the tsh daemon (the server) and the Electron app (the client).


## Request-response cycle

The usual callstack of a request handler would go through these packages:

1. `apiserver/handler`
2. `daemon`
3. `clusters`
4. `gateway` (if the request concerns a gateway)

### `apiserver/handler`

This is the package that directly implements the gRPC service, `TerminalService`. Each request has a
corresponding method defined on `handler.Handler`.

This layer shouldn't implement business logic itself but rather forward the request params somewhere
else – this helps with code reusability and tests as we don't have to mock the whole gRPC service
just to test the business logic.

### `daemon`

`daemon.Service` has access to `clusters.Storage`. `clusters.Storage` is able to list root clusters
based on profiles saved in the tsh home directory. It can also resolve [URIs](#URI) pointing at
clusters and cluster resources.

`daemon.Service` also holds a list of gateways which is a Connect-specific term for ALPN proxies
(see [RFD 39](/rfd/0039-sni-alpn-teleport-proxy-routing.md)). At this time, Connect supports only
database proxies but in the future we might add support for other
resources.

This is where `apiserver/handler` delegates most of the work to. But it might not be necessary for
every request to go through `daemon.Service`, more on that in
[#14011](https://github.com/gravitational/teleport/issues/14011).

### `clusters`

`clusters.Cluster` is the only place which has access to an instance of `TeleportClient`. Naturally,
it implements almost all of the important logic. You may notice that for each file in
`apiserver/handler` there's a corresponding file in `clusters`. This lead to `clusters.Cluster`
having a quite large surface area which in turn causes problems when writing tests for this layer.
See [#13278](https://github.com/gravitational/teleport/issues/13278) for more details.

An instance of `clusters.Cluster` is created for each incoming request. This also means that we
create a new instance of `TeleportClient` for each request to tsh daemon's gRPC server.

### `gateway`

This package deals with the intricacies of running ALPN proxies for different cluster resources. At
this time, Connect supports only database proxies but in the future we might add support for other
resources.

The term "gateway" was used to avoid using the term "proxy" which already had multiple meanings in
our codebase.

## tsh directory

At the moment, Connect uses a separate tsh home directory but we plan to [make Connect use the
default one](https://github.com/gravitational/webapps.e/issues/295).

## URI

Connect uses its own system of URIs to identify profiles, clusters, and cluster resources. This lets
us encode the location of a resource, its type, and ID in a single identifier.

```
The root cluster under the `example.cloud.gravitational.io` profile:
/clusters/example.cloud.gravitational.io

The leaf cluster `orange` under the `example.cloud.gravitational.io` profile:
/clusters/example.cloud.gravitational.io/leaves/orange

The database server `sales-production` under the `teleport.dev` profile:
/clusters/teleport.dev/dbs/sales-production

An SSH node under the leaf cluster `orange` of the `example.cloud.gravitational.io` profile:
"/clusters/example.cloud.gravitational.io/leaves/orange/servers/4bbde2c9-b805-4dd8-8032-a0bed162d85b"
```

See [`lib/teleterm/api/uri/uri.go`](/lib/teleterm/api/uri/uri.go) for the implementation details.


## Cluster lifecycle

For root clusters, the app only recognizes those that have corresponding files in the tsh home
directory. Clicking "Connect" in the app and providing the proxy address creates just the profile
yaml file in the tsh directory – this makes the profile show up in the profile list in the app.

At this point, the cluster is present in the list but not considered to be connected. Only after
logging in its state is changed to connected. It will remain as connected until the certs expire and
the user restarts the app or clicks the "Sync" button in the cluster tab.

The connection status of a leaf cluster depends on information about the leaf cluster that Connect
obtains from the root cluster.

## gRPC server

The proto files live in `proto/teleport/lib/teleterm`. `make grpc` generates protobufs in
`gen/proto`.

## Refactoring efforts

* [#14011](https://github.com/gravitational/teleport/issues/14011) Remove `daemon.Service` methods
  that only delegate work to `clusters.Cluster`.
* [#13278](https://github.com/gravitational/teleport/issues/13278) Refactor lib/teleterm/clusters
  structs to make testing easier.
