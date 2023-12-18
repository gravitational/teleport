---
authors: Tim Ross (tim.ross@goteleport.com)
state: draft
---

# RFD 126 - Resource Migrations

## Required Approvers

- Engineering @zmb3 && (@fspmarshall || @espadolini )
- Product: ( @klizhentas || @russjones )

## What

Providing support for running different versions of Auth simultaneously.

## Why

The upgrade process requires scaling Auth down to a single instance to ensure that migrations are only performed once as
well as preventing a new version and an old version from operating on the same keys with different schema. This is
cumbersome and makes the upgrade process a pain point for cluster admins. Scaling Auth down and back up also results in
an [uneven load](https://github.com/gravitational/teleport/issues/7029) which can cause connectivity issues and backend
latency that can result in a thundering herd.

Migrations are applied during Auth initialization and prevent the instance from becoming ready and able to serve
requests which causes downtime. To date, migrations have largely been applied in place on an existing key range, even
for breaking changes, which prevents an older versions of Auth from operating on the data. This can result in data loss
as new and older versions try to write different versions of a resource to the same key or worse, rendering older
versions of Teleport unable to comprehend the data stored in the backend.

## Details

Instead of all migrations being a one-shot operation which break compatibility with previous versions of Auth we can
leverage a phased approach which separates backend changes from application logic changes to allow Auth vN and Auth vN-1
to coexist. This approach is, however, likely best suited for Cloud where we control the cadence and order of Teleport
releases that are applied. Self-hosted customers should still follow the traditional upgrade procedure.

To prevent having to split the changes across separate commits and ensure that they land in the correct release, we can
leverage feature flags to allow all the changes to land at once and toggle the behavior when appropriate. Once the
migration has been completed, the feature flags and old code can be removed. For self-hosted customers migrations should
still only happen during a major release, which means major/breaking changes should not be backported.

Any Cloud tenants which are lagging behind the current release and are required to jump several versions to land on the
current release should revert to using recreate instead of a rolling deploy. We can also add a flag that indicates
migrations should be performed during initialization instead of in the background like during the phased approach. While
this will negate the changes described in this RFD to eliminate downtime, this kind of jump in releases should be the
exception and not the norm.

### Major/Breaking changes

For major breaking changes (i.e., splitting a resource into multiple resources), resources must be migrated to a new
key. For example, to migrate the data stored in `some/path/to/resource/<ID>` we must leave the original value at
`some/path/to/resource/<ID>` as is, and write the migrated value to `some/new/path/to/resource/<ID>`. Keys should be
versioned to determine which version of a resource exists at any given key range. For example, if nodes were migrated
from `/nodes/default/<UUID>` it should be to `/nodes/v2/default/<UUID>`.

Any breaking changes to a resource must also result in a major version bump to its corresponding API. For example, if we
were to make a breaking change to
[`types.LoginRule`](https://github.com/gravitational/teleport/blob/336518e0b51e118679c13a9a3a34dcff0fc8b5ad/api/proto/teleport/loginrule/v1/loginrule.proto)
then we would also need to bump the package of the
[`types.LoginRuleService`](https://github.com/gravitational/teleport/blob/336518e0b51e118679c13a9a3a34dcff0fc8b5ad/api/proto/teleport/loginrule/v1/loginrule_service.proto)
from v1 to v2.

To perform the migration which allows for backward compatibility with the previous version, each step of the migration
will be done in a phase. Each phase can be gated by a feature flag which is updated to the next phase in the process on
subsequent Cloud releases. When all the phases have been completed the code can be cleaned up to remove the transitional
state and feature flags required to get to the target state. The compatibility guarantees of this approach do come with
a cost though: it will now take longer and require more process to perform the migration. We can however mitigate the
time required to progress through each phase by increasing the Cloud release cadence.

| Phase | Write                  | Read                                        | Details                                                                       |
| ----- | ---------------------- | ------------------------------------------- | ----------------------------------------------------------------------------- |
| 1     | Old key AND new key(s) | Old Key                                     | Introduce new resource(s) and mirror data to new key range on write           |
| 2     | Old key AND new key(s) | New Key, fallback to old key if not present | Start reading from the new key and using the old key as a fall back           |
| 3     | Old key AND new key(s) | New Key, fallback to old key if not present | Run data migration that copies all keys from old to new                       |
| 4     | New key(s)             | New Key                                     | Stop mirroring writes and falling back to old resources when they don't exist |
| 5     | New key(s)             | New Key                                     | Run optional migration that removes all old keys if they do not have a TTL    |

#### Data Migration Phase

Data migrations no longer need to occur block Auth initialization which will reduce the time it will take a new Auth
instance to start processing requests. By waiting until Phase 3 to run data migrations when resource mirroring and
fallback reads are already being performed by all Auth instances the migration can happen in the background.

This phase is only required for resources which do not heartbeat. For any presence related resources we can use the
write caused by the next heartbeat to perform the data migration.

> The above only applies when doing phased migrations. Major releases used by self-hosted customers will still be using
> the one-shot migration approach will still require running the migration during initialization.

For example, `auth.Init` can discover if running in a Cloud environment via the modules package and launch all
migrations in an async manner like so:

```go
func Init(cfg InitConfig, opts ...ServerOption) (*Server, error) {
  ...


  if modules.GetModules().Features().Cloud {
    go performMigrations()
  } else {
    performMigrations()
  }

  ...

}

```

#### Feature Flags

> Note: these should only be applied within Cloud where we have explicit control over the upgrade sequence

Leveraging feature flags allows the code that handles the migrations to be decoupled from when the migrations actually
occur. Flags should be defined and provided in an environment variable so that the flags can be toggled without having to publish a new version of Teleport.

The following is an example backend service to persist `Foo` resources that uses feature flags to read/write new and/or
old keys based on the flag.

```go
func (s *FooService) GetFoo(ctx context.Context, name string) (*Foo, error) {
	switch fooFlag {
	case FooUseOldKeyOnly: // Pre-migration phase
		return s.getoldFoo(ctx, name)
	case FooMirrorWritesOnly: // Phase 1
		return s.getoldFoo(ctx, name)
	case FooMirrorWritesAndFallbackRead: // Phase 2
		f, err := s.getNewFoo(ctx, name)
		if trace.IsNotFound(err) {
			return s.getOldFoo(ctx, name)
		}
		return f, trace.Wrap(err)
	case FooUseNewKeyOnly: // Phase 4
		return s.getNewFoo(ctx, name)
	default:
		return nil, trace.BadParameter("invalid flag %v", fooFlag)
	}
}

func (s *FooService) UpsertFoo(ctx context.Context, f *Foo) error {
	switch fooFlag {
	case FooUseOldKeyOnly: // Pre-migration phase
		return s.writeToOldKey(ctx, f)
	case FooMirrorWritesOnly, FooMirrorWritesAndFallbackRead: // Phase 1 + Phase 2
		if err := s.writeToNewKey(ctx, f); err != nil {
			return trace.Wrap(err)
		}

		return trace.Wrap(s.writeToOldKey(ctx, f))
	case FooUseNewKeyOnly: // Phase 4
		return s.writeToNewKey(ctx, f)
	default:
		return nil, trace.BadParameter("invalid flag %v", fooFlag)
	}
}
```

Transitioning between phases would be controlled via the Cloud platform by configuring Teleport appropriately. It is the
author of the migrations responsibility to create a PR in Cloud which sets the correct phase. Each PR would be reviewed
and go through the same testing process as any other change made to the Cloud platform. Once all the phases have been
completed follow-up PRs to Teleport and Cloud can be made to remove the feature flags and only use the behavior of the
end state and to remove the configuration settings from needed to toggle phases.

### Minor/Additive changes

It's quite common to add a new field to an existing resource in order to support a new feature. All additive fields must
be optional - adding a field which has a required and meaningful default value can cause more issues than just lossy reads
and writes. Since the versions of instances in the cluster may lag behind Auth considerably, any new fields which need
an explicit default value to operate will cause problems for older agents which will not know about the value; this is
particularly true for Roles when adding a new field which determines access to a resource. If a default value is automatically
applied to a new field it must have no impact on the existing behavior of that resource for new and old instances of Teleport. 
Any default values which alter how old and new instances of Teleport interpret a resource are not permitted.

If a client attempts to update a resource with the new field during a rolling deployment when both the old and new
versions of Auth are running simultaneously there is a 50/50 chance that writes and reads which contain the new field
will be lossy since the old instance doesn't know about the new field. However, data loss is a problem outside of
rolling Auth updates. Reads are going to be lossy unless the client version is at the same version or newer than Auth
because they won't know about new optional fields. Writes are going to be lossy unless Auth is at the same or newer
version than clients because it won't know that there are additional fields to persist. Until there is a solution for
data loss under normal operation of a cluster we should accept the possible data loss during the very small time frame
which a rolling deployment occurs.

The risks of data loss from new fields may be mitigated in the future by the Cloud First Deployment strategy defined in
RFD 134 and automatic client updates. If Cloud is running off of master the probability of a client attempting to
read/write a field that Auth doesn't know about is less likely. Ensuring that clients are using the appropriate version
for a cluster will prevent client drift that results in data loss. As long as the clients are not upgraded until after
Auth then there is no opportunity for new fields to be accessed.

### Changing the meaning of a field

Altering or extending the capabilities of a field will result in backward incompatible behavior. Imagine the following
resource:

```protobuf
message Foo {
  repeated Bar Bars = 1;
}

message Bar {
  string Kind = 1;
  string Namespace = 2;
  string Name = 3;
}
```

Adding a new kind of `Bar` in a newer version will force the old Auth instance to make a decision about how to handle
unknown Bar kind. This is particularly troublesome when the resource is involved in access decisions. The only
reasonable decision the old Auth server can make in this case is to deny access to prevent any possible bypasses as a
result of not knowing how to process the new Bar.

This is also true for fields which are just scalar types, adding a new value to a field that is a string will result in
older instances being unable to parse the field. For example, there are several resources which take a predicate
expression defined in a string; if we were to extend the supported expressions by adding new function(s) it would result
in older instances being unable to parse the predicate expression and possibly prevent access to a resource that users
should be granted access to.

Altering the meaning of an existing field must not be accompanied by an automatic migration. These types of changes should 
require explicit opt-in from a user. For example, when adding a new predicate function Teleport should not start using
the function until the user explicitly updates their resource to use the new function. When documenting the new function
the version compatibility and consequences of using the new function in an incompatible configuration should be noted. 

### Backward Compatibility

Migrations have historically been backward incompatible operations. Migrations altered the data in place without
changing the key or resource version, which can prevent any versions prior to the migration from being able to unmarshal
the value into the correct representation. The only way to downgrade in this scenario was to restore the backend from a
backup prior to the migration, attempt to manually roll back the migration, or deleting the entire key range that was
migrated. By using the phased approach we can guarantee that no two subsequent releases of Teleport are incompatible.

### Testing Migrations

While the framework laid out in this RFD allows migrations to be applied in a deterministic manner, it does not provide
a uniform rule or process for any code that is impacted by a migration. To ensure that a migration is functional testing
should consider a wide range of simultaneous versions in a cluster in accordance to our version compatibility matrix.
Imagine that we are going to introduce a migration in v3.0.0, we must test the following for an extended period of
time(10m) to ensure all supported versions are functional:

| Auth 1 | Auth 2  | Proxy   | Agents  |
| ------ | ------- | ------- | ------- |
| v3.0.0 | v3.0.0  | v3.0.0  | v3.0.0  |
| v3.0.0 | <v3.0.0 | <v3.0.0 | <v3.0.0 |
| v3.0.0 | v3.0.0  | <v3.0.0 | <v3.0.0 |
| v3.0.0 | v3.0.0  | v3.0.0  | <v3.0.0 |

Testing multiple versions of Auth at the same time will help validate that the migration is backward compatible and that
a rollback is possible. Ensuring that Auth running with the migration and all other instances without the migration is
also crucial to test since Auth is always the first component updated. If the migration is unknown by the agents it
should not impact their ability to operate.

### Security

Migrations already exist today, this RFD only proposes a way to make them backward compatible.

### Observability

The existing `teleport_migrations` metric will be reused to record when a migration has been performed. Tracing will
also be added with a root span created by the migration framework and a child span per migration performed.

### Alternate Options Considered

#### Disallow unknown fields

> This was rejected due to the inability to accurately detect unknown fields in RPCs and elevated risk of bricking a
> cluster when rolling back due to being unable to read a resource from the backend.

We can explicitly opt-in to rejecting any backend reads or RPC requests which have unknown fields. Backend reads can
determine unknown if a stored resource has unknown fields by enabling `DisallowUnknownFields` on the json decoder. RPC
requests can try to identify unknown fields by inspecting the `XXX_unrecognized` field from each message in the request.

Examining the `XXX_unrecognized` may also falsely identify an unknown field if an item was reserved and removed from a
message instead of being deprecated and left in place. Using stricter json decoding would also cause issues with
rollbacks since any backend resource with an unknown field would be unable to be unmarshalled from the stored
representation. This would also break any agents or clients running a newer version of teleport than the Auth service
since they may have a version of `api` that contains updated proto messages.

#### Stricter Resource Versioning

> This option was rejected due to its scope and complexity. The machinery required for this option to work would take a
> long time and be a massive overhaul. Changing resource versioning to user a [major].[minor] schema would also make
> resource versions even more confusing for users.

Without a more concrete strategy for resource versioning, it is impossible to have different versions of Auth running
concurrently. Auth needs to be aware of the exact version of a resource that clients are requesting to determine if the
version of said resource stored in the backend is at the same version, if the stored version is capable of being
downgraded to that version, if the stored version is capable of being updated to the requested version, or whether the
request cannot be honored due to version incompatibility.

The only backend operation that uses a locking mechanism is `CompareAndSwap` which means that concurrent writes to the
same resource always result in the last write winning and potentially losing data. This also allows migrations to
unknowingly be reverted if an older version of Auth is able to overwrite an already migrated resource.

We need to ensure that:

1. All backend operations are applied in the order they are received, any outdated requests are rejected.
2. Auth servers know the exact version of a resource requested by clients.
3. Once a migration has been performed, Auth servers that cannot understand the migrated resource version cannot
   overwrite the resource.
4. Migrations are always applied in the correct order and are not skipped.
5. Migrations can be rolled back without having to manually edit the backend.

##### Resource versioning

The version of a resource MUST be bumped when changes are made to it. Any changes to a resource which can be converted
into the previous version and do not cause any backward compatibility issues with older Teleport instances only need to
bump the minor version of a resource. All changes to a resource which would cause backward incompatibility with other
Auth servers trying to read/write the resource MUST update the major version. All changes to a resource which alter how
that resource is understood, interpreted, and acted upon by Teleport instances MUST update the major version.

For example, if we originally have a resource like the following at v1:

```proto
syntax = "proto3";

message Foo {
  int32 bar = 1;
}
```

Then adding a new optional field which was handled appropriately, and defaulted to the correct value by application code
if empty only requires bumping the version to v1.1 and not require a direct migration.

```diff
message Foo {
  int32 bar = 1;
+  string baz = 2;
}
```

However, if we were to convert `baz` from a string to `Baz`, that change would not be easily converted into the shape of
`Foo` at v1.1 and would render the new resource unusable by clients that are only aware of `Foo` v1.1. So, the version
of the resource must be bumped from v1.1 to v2 to indicate to clients that a breaking change occurred.

```diff
message Foo {
  int32 bar = 1;
- string baz = 2;
+ reserved 2;
+ reserved baz;
+ Baz baz2 = 3;
}

+ message Baz {
+  string qux = 1;
+  int32 quux = 2;
+ }
```

To date, Auth assumes all client requests are for the version of the resource that Auth is aware of. However, this
causes problems when a resource version is bumped with a breaking change since it is not guaranteed that all Teleport
instances in a cluster are running the same version as Auth. The solution has been to downgrade the resource to the
previous version or alter the resource based on the version of the requester as indicated by the `version` header.

For clients to better communicate which version of a resource they can support there are few options:

1. Add a header that clients must populate with their greatest known version of a resource
2. Version the API such that the version is implied

For major breaking changes a new version of the API is likely warranted, however for smaller changes to a resource it
may be possible to convert the resource into the requested resource version.

This resource versioning scheme will allow Auth servers to determine which resources it knows how to read and write and
which resources it can provide read only access to (possibly with conversion) but it cannot overwrite. Any request which
is honored but causes a conversion to a lower version of a resource MUST alter the resource version of the resource
returned to end with `+downgraded` so that clients can determine how to proceed. The following tables illustrate the
scenarios in which the resource version may vary along a request route and what the outcome of each request will be.

<details open><summary>Rules for writing resources</summary>

| Stored Version                    | Write Version   | Auth Version | Client Version | Force | Outcome                                                         |
| --------------------------------- | --------------- | ------------ | -------------- | ----- | --------------------------------------------------------------- |
| v1.1                              | v1.1            | v1.1         | v1.1           | no    | OK                                                              |
| v1.2                              | v1.1            | v1.1         | v1.1           | no    | ERR (refuse to overwrite new/unknown version)                   |
| v1.2                              | v1.1            | v1.1         | v1.1           | yes   | OK                                                              |
| v1.1                              | v1.2            | v1.2         | v1.2           | \*    | OK                                                              |
| v1.1                              | v1.2            | v1.1         | v1.2           | \*    | ERR (auth never writes a version it doesn't understand)         |
| \*                                | v1.1+downgraded | \*           | \*             | no    | ERR (always refuse to write \*+downgraded)                      |
| \*                                | v1.1+downgraded | v1.1         | \*             | yes   | OK (written as v1.1, metadata stripped)                         |
| \*                                | v1.1+downgraded | v1.0         | \*             | yes   | ERR (always refuse to write unknown version, even with --force) |
| /key1/v1.1+downgraded && /key2/v2 | \*              | v1.1         | \*             | no    | ERR (always refuse to write \*+downgraded)                      |
| /key1/v1.1+downgraded && /key2/v2 | \*              | v1.1         | \*             | yes   | OK (written as v1.1, metadata stripped, /key2 is unmodified)    |
| /key1/v1.1+downgraded && /key2/v2 | v1              | v2           | \*             | \*    | OK (written to both /keyv1/v1.1+downgraded, and /key/v2)        |
| /key1/v1.1+downgraded && /key2/v2 | v2              | v2           | \*             | \*    | OK (written to both /keyv1/v1.1+downgraded, and /key/v2)        |

- Stored Version: the version of the resource stored in the backend; multiple keys denotes that the resource was
  recently migrated
- Write Version: the version of the resource to be written into the backend
- Auth Version: the default version of the resource of Auth processing the request
- Client Version: the version of the resource requested by the client
- Force: whether or not the write is forced(e.g tctl create --force)
- Outcome: the result of the operation
</details>

<details open><summary>Rules for reading resources</summary>

| Stored Version                    | Auth Version | Client Version | Outcome                                                |
| --------------------------------- | ------------ | -------------- | ------------------------------------------------------ |
| v1.2                              | v1.1         | v1.1           | OK (version=v1.1+downgraded)                           |
| v1.2                              | v1.1         | v1.2           | OK (version=v1.1+downgraded)                           |
| v2                                | v1.\*        | v1.\*          | ERR (auth cannot auto-downgrade unknown major version) |
| v2                                | v1.\*        | v2             | ERR (auth cannot auto-downgrade unknown major version) |
| v1.1                              | v1.1         | v1             | OK (version=v1+downgraded)                             |
| v1.1                              | v1.1         | v1.2           | OK (version=v1.1)                                      |
| v1.1                              | v1.1         | v2+            | OK (version=v1.1)                                      |
| /key1/v1.1+downgraded && /key2/v2 | v1.1         | v1.1           | OK (version=v1.1+downgraded)                           |
| /key1/v1.1+downgraded && /key2/v2 | v1.1         | v2             | OK (version=v1.1+downgraded)                           |
| /key1/v1.1+downgraded && /key2/v2 | v2           | v2             | OK (version=v2)                                        |

- Stored Version: the version of the resource stored in the backend; multiple keys denotes that the resource was
  recently migrated
- Auth Version: the default version of the resource of Auth processing the request
- Client Version: the version of the resource requested by the client
- Outcome: the result of the operation

</details>

##### Optimistic Locking

The backend will be updated to support optimistic locking in order to prevent two simultaneous writes to a resource from
overwriting one another. The resource metadata shall have a new `Revision` field that will include a backend specific
opaque identifier which will be used to reject any writes that do not have a matching `Revision` with the existing item
in the backend. The `Revision` of a resource should not be altered by or counted on to be deterministic by clients, they
should treat the field as an opaque blob and ignore it.

<details open><summary>Metadata changes</summary>

```diff
message Metadata {
  // Name is an object name
  string Name = 1 [(gogoproto.jsontag) = "name"];
  // Namespace is object namespace. The field should be called "namespace"
  // when it returns in Teleport 2.4.
  string Namespace = 2 [(gogoproto.jsontag) = "-"];
  // Description is object description
  string Description = 3 [(gogoproto.jsontag) = "description,omitempty"];
  // Labels is a set of labels
  map<string, string> Labels = 5 [(gogoproto.jsontag) = "labels,omitempty"];
  // Expires is a global expiry time header can be set on any resource in the
  // system.
  google.protobuf.Timestamp Expires = 6 [
    (gogoproto.stdtime) = true,
    (gogoproto.nullable) = true,
    (gogoproto.jsontag) = "expires,omitempty"
  ];
  // ID is a record ID
  int64 ID = 7 [(gogoproto.jsontag) = "id,omitempty"];
+  Revision is an opaque identifier used to enforce optimistic locking.
+  string Revision = 8 [(gogoproto.jsontag) = "revision"];
}
```

</details>

In other words, when a resource is written the `Revision` is altered by the backend. However, prior to the resource
being written the `Revision` of the new value and the existing resource in the backend are compared, if they match then
the update is permitted, if they differ then the update is rejected. So, if two clients try to update the same value
concurrently, only the first write will succeed and the second will be rejected. The second client will have to fetch
the resource, apply their change and try to update again.

The backend interface will be extended to support new conditional delete and update methods which enforce optimistic
locking. Most if not all user facing and editable resources should use the new optimistic locking primitives to prevent
losing changes made by a user. Resources which are updated based on presence are likely not a good candidate for
conditional operations due to the amount of stress that may put on backends.

<details open><summary>Backend changes</summary>
g) (*Lease, error)
}
```

</details>

##### Migration Strategy

In addition to migrating keys as outlined above, when a migration converts a resource at the new key, the corresponding
resource at the old key must also have its version appended with `+downgraded`. For example converting a resource at
v1.1 to v2 via a migration should update the old resource to now have a version of `v1.1+downgraded`. This is an
indication to older Auth servers that the resource is now read only.

#### Deferred Migrations

> This was rejected due to challenges detecting when the right time to perform migrations is. Relying on presence
> information may be inaccurate and result in the migration starting prematurely. The Cloud only variant which relied on
> coordination between Tenant Operator and Auth would not scale without an authenticated control interface available for
> the two components to communicate over.

To prevent any migrations from impacting older versions of Teleport we can defer execution of migrations until all
versions of Auth are running a version which understands the migration. Determining peer versions is not possible today,
but could be done via monitoring Auth resources within Auth and waiting until all older versions disappear. Detection of
old Auth servers may take some time and not be a reliable picture of the cluster if heartbeats are stale, or a new Auth
server was only online long enough to heartbeat and then was terminated. Without an Auth peering mechanism detection of
different Auth instances within a cluster may not be reliable.

We could pursue a Cloud only variant of this strategy which relies on the Tenant Operator to signal to Auth when all
older versions of Auth have been terminated. However, there is currently no control interface that allows Tenant
Operator to communicate with Teleport Auth or Proxy. We could use signals, however, it would have to be a custom
real-time signal since Teleport already consumes all of the standard signals for other purposes. To avoid putting
pressure on kubernetes we should avoid anything that requires "kube exec" as this won't scale well.

This strategy also does not prevent application logic from having to be aware of both the old and new resources since
there is no guarantee that the migrations will be executed immediately. Any new features which rely on the new resource
will either temporarily not function or need a compatibility mode to prevent the new application logic from kicking in
until the migration has completed.
