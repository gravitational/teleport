---
Authors: Maxim Dietz (maxim.dietz@goteleport.com)
State: draft
---

# RFD 230 - Component Feature Advertisement

## Required Approvers
- Engineering: (@r0mant && @rosstimothy)

## Related
- [RFD 228](https://github.com/gravitational/teleport/blob/master/rfd/0228-resource-scoped-constraints-in-access-requests.md) - Resource-Scoped Constraints in Access Requests
- [RFD 225](https://github.com/gravitational/teleport/blob/master/rfd/0225-teleport-extended-support.md) - Teleport Extended Release Support

## What
This RFD introduces a small feature advertisement payload carried on the presence resources that services already
heartbeat to Auth (e.g., `ServerV2` for SSH, `AppServerV3` for app access, and the corresponding types for
database and Kubernetes access).

Auth persists these advertised features alongside the existing presence resources and exposes them through its existing
APIs. Consumers such as Proxy can read these feature sets and intersect them across a request path to
decide whether to enable behavior that depends on a given capability.

The initial application is for App Service and Auth to signal support for Resource Constraints in certificates
([RFD 228](https://github.com/gravitational/teleport/blob/master/rfd/0228-resource-scoped-constraints-in-access-requests.md)),
so that Web can hide Resource Constraint–dependent flows when the required support is not present.

### Terminology
In this RFD:
- **Service** means a Teleport agent/service that reports its presence to Auth, such as App Service, SSH/Node Service,
  Database Service, or Kubernetes Service. These are the processes that advertise their feature support via presence
  resources and heartbeats.
- **Consumer** means a process that reads feature support and derives higher-level behavior from it, such as
  Proxy/Web UI and Auth.

Auth plays both roles:
- As a service, it exposes its own `ComponentFeatures` via its heartbeat.
- As a consumer, it stores and serves the feature sets advertised by other services, using them internally and
  surfacing them to other consumers.

> [!IMPORTANT]
> This mechanism is intended as a non-security signaling layer, with its initial use being UX/API gating and
> coordination between services and consumers to ensure backwards/forwards compatibility with changes required for
> [RFD 228](https://github.com/gravitational/teleport/blob/master/rfd/0228-resource-scoped-constraints-in-access-requests.md).
> It does not replace secure design or fail-closed behavior. All security-relevant behavior MUST remain correct in the
> absence of flags and MUST fail closed. Treat flags as hints for conditional UX or workflows, not as authorization
> signals.

## Why
Relying solely on version numbers to determine feature support is fragile, especially in a distributed system with
multiple services that can be upgraded independently and where certain features may be backported to older versions.

Blindly showing UI elements for features that backing services do not support leads to a frustrating user
experience, even if errors are handled gracefully. This mechanism provides an explicit, composable signal consumers can
use to "fail silently" when certain capabilities are not available and hide unsupported flows from the get-go.

## Design
### Payload format
The payload is simple and forward-compatible by design. In v1, features are Boolean: presence means "supported", absence
means "not supported". Each feature is identified by an append-only enum value so IDs remain stable over time.
```protobuf
enum ComponentFeatureID {
  COMPONENT_FEATURE_ID_UNSPECIFIED             = 0;
  // v1 addition: Support for RFD 228 ResourceAccessIDs and Resource Constraints
  COMPONENT_FEATURE_ID_RESOURCE_CONSTRAINTS_V1 = 1;
}

message ComponentFeatures {
  repeated ComponentFeatureID features = 1;
}
```

Unknown feature IDs can be ignored by consumers (optionally logged once per service identity). Because senders only
include "true" values, the payloads stay compact, and omission naturally encodes "false"/"not supported".

### Advertising from services
Each service that advertises features exposes a list of its current `ComponentFeatures`. For heartbeat-ing services,
this payload is attached to the presence resources they already send over the inventory control stream (e.g.,
`AppServerSpecV3` for App Service). No changes are required in the heartbeat
machinery itself; services simply add a `ComponentFeatures` field to the Spec of the presence resources they are
already constructing.

A service should advertise a flag only when:
- the implementation is present in the build, and
- it is usable with the current configuration and dependencies.

For the initial Resource Constraints work, support is not configuration-dependent. If the code path is compiled into the
binary, the service can unconditionally advertise the flag for the resources that participate in that flow.

Auth will stores these payloads alongside existing status information so they are available through the same APIs that
already expose presence and cluster state.

Auth itself won't carry `ComponentFeatures` on an auth-server presence resource. Instead, it can expose its own feature
support via adding some `AuthComponentFeatures` field to its Spec when heartbeating.

### Consuming feature information
On the consumer side, the logic is straightforward. For a target operation, the consumer gathers the participating
services (for example, Auth and the App Service that will serve an app session) and intersects their feature sets to
determine feature support.

In practice,
- Consumers obtain per-service `ComponentFeatures` by reading presence resources from Auth (e.g., app servers,
  database servers, and nodes).
- If the feature requires auth support, consumers also obtain all present Auth instances (e.g., via
  `accessPoint.GetAuthServers()` in Web handlers), which contain Auth's own `ComponentFeatures` on their Specs.
- They then compute an intersection of these sets for the path or resource in question and gate UX or other
  behavior on that intersection.

The Web UI should not need to deal with raw feature IDs directly. Proxy should evaluate the intersection logic and
derive higher-level capability bits that are exposed to consumers (such as Web UI) via existing APIs. If Proxy does not
implement this logic, these will never be set, and Web UI will behave as "feature unsupported" implicitly.

### `tctl` visibility
Each service's `ComponentFeatures` should be surfaced via `tctl inventory list` / `tctl inventory get` (e.g., as a
comma-separated `features` column and in the JSON/YAML output). This gives CLI users a simple way to confirm which
agents support a capability and to debug mixed-version rollouts.

### Scope of v1
In v1, the implementation is limited to:
- App Service advertising `ComponentFeatures` on `AppServerV3` presence records for AWS Console applications.
- Auth and Proxy exposing their own `ComponentFeatures` via their `ServerSpecV2` when heartbeating.
- Proxy/Web UI consuming these feature sets to gate Resource Constraint–dependent flows.

The design is intentionally generic so that other services can adopt `ComponentFeatures` for future features without
changing any core model.

### Example: Resource Constraints
The initial use-case is Resource Constraints in certificates
([RFD 228](https://github.com/gravitational/teleport/blob/master/rfd/0228-resource-scoped-constraints-in-access-requests.md)):
- Auth must accept `ResourceAccessID`s carrying `ResourceConstraints` in Access Requests, embed them into and parse them
  from X.509 `tlsca`/`sshca` Identity certificates, and enforce them via `AccessChecker` when issuing AWS Console
  sessions.
- App Service does not implement any Resource Constraint logic itself at this point; it is responsible for proxying
  application traffic. However, only some resource kinds (initially, only AWS Console Application resources) should
  participate in Resource-Constraint-related flows, so App Service must advertise support accordingly.
- Web must only expose Resource Constraint-related flows when the rest of the path can honor them.

#### App Service: advertise via `AppServer` heartbeats
The App Service already uses `HeartbeatV2` to send `AppServerV3` resources for each proxied app:
```go
heartbeat, err := srv.NewAppServerHeartbeat(srv.HeartbeatV2Config[*types.AppServerV3]{
	InventoryHandle: s.c.InventoryHandle,
	GetResource: s.getServerInfoFunc(app),
	OnHeartbeat: s.c.OnHeartbeat,
})
```

`GetResource` builds the `*types.AppServerV3` that `appServerHeartbeatV2.Announce` wraps in
`InventoryHeartbeat{AppServer: ...}` and pushes over the inventory control stream.

To advertise feature support from App Service, `AppServerSpecV3` is extended with a new field carrying a list of
`ComponentFeatureID`s:
```protobuf
message AppServerSpecV3 {
  // existing fields...
  ComponentFeatures component_features = N;
}
```

For Resource Constraints, this flag is per-application, not per-process. Only AWS Console apps (initially) are
eligible for Resource Constraints. `getServerInfoFunc()` is extended so that the `AppServerV3` it returns marks only
those apps as supporting the feature:
```go
func (s *Server) getServerInfoFunc(app types.Application) func(ctx context.Context) (*types.AppServerV3, error) {
    return func(ctx context.Context) (*types.AppServerV3, error) {
        server, err := s.buildAppServerV3(app) // existing metadata / labels / spec
        if err != nil {
            return nil, trace.Wrap(err)
        }

		if app.IsAWSConsole() {
            server.Spec.ComponentFeatures = &presence.ComponentFeatures{
                Features: []presence.ComponentFeatureID{
                    presence.COMPONENT_FEATURE_ID_RESOURCE_CONSTRAINTS_V1,
                },
            }
        }

        return server, nil
    }
}
```

As multiple app servers may serve a single application, the aggregation logic used in `UnifiedResourcesCache` should
be updated to intersect the `ComponentFeatures` of all `AppServerV3` records backing a given application, e.g.:
```go
// New wrappers around types.AppServer to hold aggregated features
type aggregatedAppServer struct {
	types.AppServer
	features *presence.ComponentFeatures
}
func (a *aggregatedAppServer) Copy() types.AppServer {
    out := a.AppServer.Copy()
    if a.features != nil {
        out.SetComponentFeatures(a.features)
    }
    return out
}
func (a *aggregatedAppServer) CloneResource() types.ResourceWithLabels {
    return a.Copy()
}

// Compute intersection of features across multiple app servers
func intersectComponentFeaturesApp(servers map[string]types.AppServer) *presence.ComponentFeatures { /*...*/ }

// Wire up in newResourceCollection
func newResourceCollection(r resource) resourceCollection {
    switch r := r.(type) {
    case types.AppServer:
        return newServerResourceCollection(r,
            func (latest types.AppServer, servers map[string]types.AppServer) types.AppServer {
                return &aggregatedAppServer{
                    AppServer: latest,
                    features:  intersectComponentFeaturesApp(servers),
                }
            },
        )
    case types.DatabaseServer:
    case types.KubeServer:
    case serverResource:
    default:
        // [existing aggregation]...
    }
}
```

No changes are needed in `HeartbeatV2` itself. It continues to call `GetResource` and sends whatever `AppServerV3` it's
given. Auth’s inventory layer persists those `AppServerV3` records and makes them visible through the presence/inventory
APIs that Proxy and `tctl` already use.

#### Auth and Proxy
Auth participates in the bulk of the Resource Constraints implementation, as it must parse and enforce
`ResourceAccessID`s in Access Requests and certificates. Unlike App Service, Auth does not heartbeat via presence; it
provides `UpsertAuthServer` as the heartbeat announcer. Extending Auth's `ServerSpecV2` to add a
`ComponentFeatures` field will allow Proxy and other consumers to read Auth's feature set via existing APIs:
```go
heartbeat, err := srv.NewHeartbeat(srv.HeartbeatConfig{
    Mode:      srv.HeartbeatModeAuth,
    Context:   process.GracefulExitContext(),
    Component: teleport.ComponentAuth,
    Announcer: authServer,
    GetServerInfo: func() (types.Resource, error) {
        srv := types.ServerV2{
            Kind:    types.KindAuthServer,
            Version: types.V2,
            Metadata: types.Metadata{
                Namespace: apidefaults.Namespace,
                Name:      connector.HostUUID(),
            },
            Spec: types.ServerSpecV2{
                Addr:     authAddr,
                Hostname: process.Config.Hostname,
                Version:  teleport.Version,
				ComponentFeatures: &presence.ComponentFeatures{
                    Features: []presence.ComponentFeatureID{
						presence.COMPONENT_FEATURE_ID_RESOURCE_CONSTRAINTS_V1,
                    },
                },
            },
        }
		// [...]
        return &srv, nil
    },
	// [...]
})
```

When serving Web APIs such as `ClusterUnifiedResourcesGet` (used by the Web UI to list resources as `UnifiedResource`
records), Proxy:
1. Uses its Auth `accessPoint` to `ListAuthServers()` and `ListProxies()`, computing the intersection of all Auth and
   Proxy servers' `ComponentFeatures`, then
2. Intersects that with each `AppServerV3` presence record backing a given AWS Console app, then
3. Sets a flag (e.g., `SupportsResourceConstraints`) from that intersection on the relevant `UnifiedResource` record.

Web UI can then check `SupportsResourceConstraints` on each `UnifiedResource`, instead of working directly with
feature IDs and/or relying on `kind == app && subKind == awsConsole`.

> [!NOTE]
> This RFD is concerned with avoiding incorrect assumptions about Auth and agent capabilities in mixed-version clusters.
> It does not attempt to solve potential version skew between Proxy and Web UI themselves; that continues to rely on
> the usual API compatibility guarantees. Adding some `GetProxyServerFeatures` endpoint that returns a
> `ComponentFeatures` payload for the current Proxy instance would also involve moving some or all of the intersection
> logic into the Web UI, which has its own trade-offs with regard to complexity.

## Compatibility
Backwards compatibility is straightforward. Services that do not yet publish a features payload are treated as
supporting no features. Older readers that encounter an unknown enum value simply ignore it, so intersections naturally
degrade to the set of features both sides understand.

Forward compatibility follows the same pattern. New enum values can be appended without affecting older readers and
intersections remain safe. If we ever need non-Boolean data or a different shape, we can introduce a new field within
`ComponentFeatures` (e.g., `FeatureDetails`).

### Feature lifecycle & deprecation
Since features are identified by stable enum values, they should be treated as effectively permanent once introduced.
If a feature becomes obsolete, it can be marked as deprecated in the enum documentation, but its ID must not be reused
or removed, and advertisements of the feature should continue.

## Caveats and guidance
**No dynamic flags:** Do not use this mechanism to advertise runtime settings or mutable state. A service’s advertised
feature set must be stable for the lifetime of the process (or at least between heartbeats without mid-cycle toggling).

**Not a config/health channel:** Do not encode configuration values, environment health, or per-tenant state as
"features". If richer or non-Boolean data is ever needed for coordination, we should introduce a separate, purpose-built
payload or evolve `ComponentFeatures` with a new field that does not change per-request.
