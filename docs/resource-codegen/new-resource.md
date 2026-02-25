# Adding a New Resource with resource-gen

## Step 1: Define the proto

Create the directory and two files under `api/proto/teleport/<resource>/v1/`:

```bash
mkdir -p api/proto/teleport/<resource>/v1/
```

**`thing.proto`** — the resource message (RFD 153 shape):

```protobuf
syntax = "proto3";

package teleport.thing.v1;

import "teleport/header/v1/metadata.proto";

option go_package = "github.com/gravitational/teleport/api/gen/proto/go/teleport/thing/v1;thingv1";

// Thing is a ... resource.
message Thing {
  // kind is the resource kind.
  string kind = 1;
  // sub_kind differentiates variants of the same resource kind.
  string sub_kind = 2;
  // version is the resource schema version.
  string version = 3;
  // metadata is common resource metadata.
  teleport.header.v1.Metadata metadata = 4;
  // spec is user-owned desired state.
  ThingSpec spec = 5;
  // status contains runtime-only state.
  ThingStatus status = 6;
}

// ThingSpec contains user-configurable thing properties.
message ThingSpec {
  // your fields here
}

// ThingStatus contains runtime-only thing state.
message ThingStatus {
  // runtime-only fields here (optional)
}
```

**`thing_service.proto`** — the gRPC service with a `resource_config` option, plus all request/response messages:

```protobuf
syntax = "proto3";

package teleport.thing.v1;

import "google/protobuf/empty.proto";
import "teleport/options/v1/resource.proto";
import "teleport/thing/v1/thing.proto";

option go_package = "github.com/gravitational/teleport/api/gen/proto/go/teleport/thing/v1;thingv1";

// ThingService provides CRUD methods for the Thing resource.
service ThingService {
  option (teleport.options.v1.resource_config) = {
    storage: {
      backend_prefix: "things"
      standard: {}
    }
  };

  // GetThing retrieves a thing by name.
  rpc GetThing(GetThingRequest) returns (Thing);
  // ListThings lists things using paginated responses.
  rpc ListThings(ListThingsRequest) returns (ListThingsResponse);
  // CreateThing creates a new thing.
  rpc CreateThing(CreateThingRequest) returns (Thing);
  // UpdateThing updates an existing thing.
  rpc UpdateThing(UpdateThingRequest) returns (Thing);
  // DeleteThing removes a thing by name.
  rpc DeleteThing(DeleteThingRequest) returns (google.protobuf.Empty);
}

// GetThingRequest is a request to fetch one thing by name.
message GetThingRequest {
  // name is the thing metadata.name value.
  string name = 1;
}

// ListThingsRequest is a request to list things.
message ListThingsRequest {
  // page_size is the maximum number of things to return.
  int32 page_size = 1;
  // page_token is the token returned from a previous list request.
  string page_token = 2;
}

// ListThingsResponse is a paginated thing list response.
message ListThingsResponse {
  // The repeated field MUST be the lowercase plural of the kind (e.g., "things").
  // resource-gen generates `rsp.Things` — the field name drives the Go getter.
  repeated Thing things = 1;
  // next_page_token points to the next page of results.
  string next_page_token = 2;
}

// CreateThingRequest is a request to create a thing.
message CreateThingRequest {
  // The resource field MUST be the lowercase kind (e.g., "thing").
  // resource-gen generates `req.GetThing()` — the field name drives the Go getter.
  Thing thing = 1;
}

// UpdateThingRequest is a request to update a thing.
message UpdateThingRequest {
  // Same naming rule as CreateThingRequest.
  Thing thing = 1;
}

// DeleteThingRequest is a request to delete a thing.
message DeleteThingRequest {
  // name is the thing metadata.name value.
  string name = 1;
}
```

The `resource_config` option is what the parser discovers. Only define the RPCs you need — the parser infers which operations exist from the RPC names (Get, List, Create, Update, Upsert, Delete). The parser also validates that request messages contain required fields (`name` for Get/Delete, `page_size`/`page_token` for List).

**Upsert** is also supported: add `rpc UpsertThing(UpsertThingRequest) returns (Thing);` with a matching `UpsertThingRequest` (same shape as `CreateThingRequest`). Upsert requires Create to also be defined.

## Step 2: Run `make grpc/host`

```bash
make grpc/host
```

This runs a four-phase pipeline:
1. **Event injection** — `resource-gen --events-only` injects event messages into `events.proto` (appends missing OneOf entries and message definitions) and watch entries into `event.proto` (import lines and `oneof Resource` entries for cache-enabled resources)
2. **Proto compilation** — `buf build`/`buf lint`/`buf generate` to compile all protos (including the newly injected event messages) into `*.pb.go` and `*_grpc.pb.go`
3. **Proto codegen** — `genproto.sh` runs additional proto code generation
4. **Go generation** — `resource-gen` produces up to 13 per-resource generated Go files (7 always-overwritten `.gen.go` + 3 cache files when `cache.enabled` is true + 3 scaffold created once, never overwritten), plus event registration files when audit events are enabled, plus a TypeScript file for web UI audit event display

After this you should have ~16 new per-resource files: proto pb files (typically 3, depending on buf plugin config) + 13 resource-gen files (with default `cache.enabled: true`).

When cache is enabled (the default), `resource-gen` also produces:
- **Watch proto entries** (auto-injected into `event.proto`): import lines and `oneof Resource` entries so the watch system can stream the new resource
- **Events client dispatch** (cross-resource, always regenerated): `api/client/events_generated.gen.go` containing `generatedEventToGRPC` and `generatedEventFromGRPC` dispatch functions, eliminating manual switch cases in `events.go`
- **Cache config helper** (cross-resource, always regenerated): `ToCacheConfig()` method on `servicesGenerated` in `lib/auth/services.gen.go`, eliminating manual `GeneratedConfig` struct literals
- **Cache test registration** (per-resource): `lib/cache/thing_register.gen_test.go` containing init()-based test resource registration for `cache_test.go` infrastructure

When audit events are enabled (the default), `resource-gen` also produces:
- **Event proto messages** (auto-injected into `events.proto`): OneOf entries and message definitions for each enabled audit operation
- **Event registration files** (cross-resource, always regenerated): event type constants in `lib/events/api.gen.go`, event code constants in `lib/events/codes.gen.go`, dynamic factory registrations in `lib/events/dynamic.gen.go`, test map registrations in `lib/events/events_test.gen.go`, and OneOf converter registrations in `api/types/events/oneof.gen.go`
- **Web UI audit event metadata** (cross-resource, always regenerated): `web/packages/teleport/src/services/audit/generatedResourceEvents.gen.ts` containing event codes, TypeScript types, formatters, icons, and fixtures for the web UI audit log viewer

By default, cache is enabled, MFA is required for mutations, and audit events are emitted on create/update/delete. See the [Configuration Reference](#configuration-reference) for all defaults and how to override them.

### Running resource-gen standalone

For faster iteration, you can run `resource-gen` directly without the full `make grpc/host`:

```bash
cd build.assets/tooling && go run ./cmd/resource-gen \
  --proto-dir=../../api/proto \
  --output-dir=../.. \
  --module=github.com/gravitational/teleport

# Preview what files would be generated without writing them:
cd build.assets/tooling && go run ./cmd/resource-gen \
  --proto-dir=../../api/proto \
  --output-dir=../.. \
  --module=github.com/gravitational/teleport \
  --dry-run
```

| File | What it is |
|------|-----------|
| `api/gen/proto/go/teleport/thing/v1/*.pb.go` | Proto-generated Go types and gRPC stubs |
| `lib/services/thing.gen.go` | `Things` + `ThingsGetter` interfaces, marshal/unmarshal helpers |
| `lib/services/thing.go` | **Scaffold** — `ValidateThing()` stub (`NotImplemented`), follows RFD 153 convention |
| `lib/services/thing_test.go` | **Scaffold** — initially-failing validation test table |
| `lib/services/local/thing.gen.go` | `ThingService` backend using `generic.ServiceWrapper` |
| `lib/auth/thing/thingv1/service.gen.go` | gRPC service with RBAC, calls `services.ValidateThing()` and `s.authorizeMutation()` (MFA logic lives in the scaffold). Also defines a `Reader` interface (Get/List) that the cache must satisfy — this is the type used by `ServiceConfig.Reader`. |
| `lib/auth/thing/thingv1/service.go` | **Scaffold** — `ServiceConfig`, `Service` struct, `NewService()`, `authorize()`, `authorizeMutation()`, audit event implementations |
| `api/client/thing.gen.go` | API client methods (`GetThing`, `ListThings`, etc.) |
| `lib/auth/thing_register.gen.go` | Auth gRPC wiring via `init()` |
| `lib/cache/thing_register.gen.go` | Cache collection wiring via `init()` |
| `lib/cache/thing.gen.go` | Cache accessor methods (`Get`/`List` on `*Cache`) |
| `lib/services/local/thing_register.gen.go` | Events parser wiring via `init()` |
| `lib/cache/thing_register.gen_test.go` | Cache test resource registration via `init()` |
| `tool/tctl/common/resources/thing_register.gen.go` | tctl handler wiring via `init()` |
| `api/client/events_generated.gen.go` | EventToGRPC/EventFromGRPC dispatch for generated resources (cross-resource) |
| `web/.../audit/generatedResourceEvents.gen.ts` | Web UI audit event codes, formatters, icons, fixtures (cross-resource) |

## Step 3: Try to compile

**Packages that compile immediately** (no further action needed):
- `lib/services/local/` — parser + backend use types from the same generation
- `tool/tctl/common/resources/` — handler uses the generated API client and service interface
- `lib/auth/` — the `servicesGenerated` struct in `lib/auth/services.gen.go` is regenerated automatically with the new resource's field, so the auth `Services` type already includes `services.Things`.
- `lib/cache/` — the `GeneratedConfig` struct in `lib/cache/index.gen.go` is regenerated automatically with the new resource's embedded interface, so the cache `Config` type already includes `services.Things`. `GeneratedConfig` is exported so external packages (like `accesspoint`) can set it in struct literals.
- `lib/auth/authclient/` — the `cacheGeneratedServices` interface in `lib/auth/authclient/api.gen.go` is regenerated automatically with the new resource's getter embed, so the `Cache` interface already includes `services.ThingsGetter`.

**Packages that should compile immediately** (no further action needed):

The **scaffold** (`service.go`) references event types from `events.proto`. These are now auto-injected by `resource-gen --events-only` as part of the `make grpc/host` pipeline, so the scaffold should compile right away.

All registration wiring is fully automatic — no manual struct/interface edits are needed. The generated registration files include compile-time static assertions that verify the wiring is complete.

## Step 4: Customize the generated code

Three scaffold files require developer attention:

### 4a. Implement validation

`lib/services/thing.go` contains:
- `ValidateThing()` — an exported validation function that returns `trace.NotImplemented()` by default. Implement checks for nil resource, nil metadata, empty name, kind mismatch, nil spec, and resource-specific field validation.

This follows the established RFD 153 convention (see `ValidateWorkloadIdentity`, `ValidateHealthCheckConfig`, etc. in `lib/services/`). The exported function is reusable across callers (gRPC service, tctl, tests, etc.).

`lib/services/thing_test.go` contains a table-driven test for `ValidateThing()` that initially fails on the "valid minimal" case (due to `NotImplemented`). Once you implement validation, this test should pass.

### 4b. Customize the service scaffold

`lib/auth/thing/thingv1/service.go` is the primary developer-owned file. It contains:

- **`ServiceConfig` struct** — standard fields (`Authorizer`, `Backend`, `Reader`, `Emitter`) plus a `// TODO: add resource-specific dependencies` marker for custom fields like `UsageReporter`, `Clock`, etc.
- **`CheckAndSetDefaults()`** — validates required config fields.
- **`Service` struct** — standard fields plus a `// TODO: add resource-specific fields` marker.
- **`NewService()`** — constructor that wires config into the service.
- **`authorize()`** — read authorization (verbs only). Override to add module gating, custom RBAC, etc.
- **`authorizeMutation()`** — write authorization (delegates to `authorize()`, adds MFA). Override for custom write-path checks.
- **`emitCreateAuditEvent()`**, **`emitUpdateAuditEvent()`**, **`emitDeleteAuditEvent()`**, and **`emitUpsertAuditEvent()`** (when Upsert RPC is defined) — fully-populated audit event implementations with `FIXME` markers for resource-specific fields. The upsert method delegates to `emitCreateAuditEvent` when the old resource is nil or `emitUpdateAuditEvent` otherwise — no separate upsert event types are needed.

The audit methods reference event types and constants. Event type string constants (e.g., `libevents.ThingCreateEvent`) and event code constants (e.g., `libevents.ThingCreateCode`) are auto-generated by `resource-gen` into `lib/events/api.gen.go` and `lib/events/codes.gen.go` respectively.

What the developer still needs to do:

1. **Fill FIXME markers** in the service scaffold (`lib/auth/thing/thingv1/service.go`) — search for `FIXME` and add any resource-specific audit event fields.

Event messages in `events.proto` (OneOf entries and message definitions) are
automatically injected by `resource-gen --events-only` as part of the
`make grpc/host` pipeline. The standard event messages include Metadata,
ResourceMetadata, UserMetadata, ConnectionMetadata, and Status embeds. If you
need resource-specific fields beyond these standard embeds, add them manually
to the injected messages in `events.proto` after the first `make grpc/host` run.

These scaffold files are **developer-owned** — `make grpc` will not overwrite them once they exist.

### 4c. Customize tctl text output

By default, tctl columns are driven by `tctl.columns` and `tctl.verbose_columns` in the `resource_config` proto option. The default is Name for standard mode and Name + Revision + Expires for verbose mode. These defaults are always applied, so the generated `WriteText` uses configured columns out of the box.

To customize columns, set `tctl.columns` and `tctl.verbose_columns` in your proto's `resource_config`:

```protobuf
option (teleport.options.v1.resource_config) = {
  storage: { backend_prefix: "things" standard: {} }
  tctl: {
    columns: ["metadata.name", "spec.flavor"]
    verbose_columns: ["metadata.name", "spec.flavor", "metadata.revision"]
  }
};
```

## Configuration Reference

The `resource_config` option supports several config sections that control generated output. All have sensible defaults — you only need to set values that differ from defaults.

| Config field | Default | Effect |
|---|---|---|
| `pagination.default_page_size` | 200 | Enforced in backend List when client sends 0 |
| `pagination.max_page_size` | 1000 | Clamped in backend List |
| `cache.enabled` | `true` | When `false`, cache files are not generated |
| `audit.emit_on_create` | `true` | When `false`, Create skips audit event emission |
| `audit.emit_on_update` | `true` | When `false`, Update/Upsert skips audit event emission |
| `audit.emit_on_delete` | `true` | When `false`, Delete skips audit event emission |
| `audit.code_prefix` | — | Prefix for event code constants. 2-4 uppercase ASCII characters (e.g., `"CK"`). Required when any `emit_on_*` is true. |
| `tctl.description` | `"<Kind> resources"` | Resource description shown in tctl help |
| `tctl.columns` | `["metadata.name"]` | Columns shown in `tctl get` text output |
| `tctl.verbose_columns` | `["metadata.name", "metadata.revision", "metadata.expires"]` | Columns shown with `--verbose` |
| `tctl.mfa_required` | `true` | Whether tctl handler requires MFA |
| `hooks.enable_lifecycle_hooks` | `false` | When `true`, generates `Hooks` struct with Before/After callbacks |
| `operations.immutable` | `false` | When `true`, Update/Upsert return "immutable" errors |
| `storage.singleton.fixed_name` | — | For singleton resources, Get/Delete use this fixed name |
| `storage.scoped.by` | — | For scoped resources, the proto field name used as the scope key (e.g., `"username"`) |

## Full example: all options

The following service proto shows every `resource_config` option set explicitly. In practice you only need `storage` — everything else has sensible defaults.

```protobuf
service GadgetService {
  option (teleport.options.v1.resource_config) = {
    // Storage (required). Pick exactly one pattern.
    storage: {
      backend_prefix: "gadgets"

      // Option A: standard — keys are gadgets/<name>
      standard: {}

      // Option B: singleton — keys are gadgets/<fixed_name>
      // singleton: { fixed_name: "current" }

      // Option C: scoped — keys are gadgets/<scope>/<name>
      // scoped: { by: "username" }
    }

    // Cache (optional). Defaults: enabled=true, indexes=["metadata.name"].
    cache: {
      enabled: true
      indexes: "metadata.name"
    }

    // Audit (optional). Defaults: emit_on_create=true, emit_on_update=true,
    // emit_on_delete=true, emit_on_get=false.
    // code_prefix is required when any emit_on_* is true.
    audit: {
      emit_on_create: true
      emit_on_update: true
      emit_on_delete: true
      emit_on_get: true
      code_prefix: "GD"
    }

    // tctl (optional). Defaults: mfa_required=true, description="<Kind> resources",
    // columns=["metadata.name"], verbose_columns=["metadata.name", "metadata.revision", "metadata.expires"].
    tctl: {
      description: "Gadget resources"
      mfa_required: true
      columns: ["metadata.name", "spec.flavor"]
      verbose_columns: ["metadata.name", "spec.flavor", "spec.color", "metadata.revision", "metadata.expires"]
    }

    // Hooks (optional). Default: enable_lifecycle_hooks=false.
    hooks: {
      enable_lifecycle_hooks: true
    }

    // Pagination (optional). Defaults: default_page_size=200, max_page_size=1000.
    pagination: {
      default_page_size: 100
      max_page_size: 500
    }
  };

  rpc GetGadget(GetGadgetRequest) returns (Gadget);
  rpc ListGadgets(ListGadgetsRequest) returns (ListGadgetsResponse);
  rpc CreateGadget(CreateGadgetRequest) returns (Gadget);
  rpc UpdateGadget(UpdateGadgetRequest) returns (Gadget);
  rpc UpsertGadget(UpsertGadgetRequest) returns (Gadget);
  rpc DeleteGadget(DeleteGadgetRequest) returns (google.protobuf.Empty);
}
```

This configuration produces:
- Standard key layout at `gadgets/<name>`
- Cache enabled with default name index
- Audit events on all four operations (create `GD001I`, update `GD002I`, delete `GD003I`, get `GD004I`)
- tctl output with custom columns including `spec.flavor` and `spec.color`
- Lifecycle hooks (`Before`/`After` callbacks for Create, Update, Upsert, Delete)
- Pagination capped at 500 per page
- Upsert support (delegates to create or update audit as appropriate)

## What you DON'T have to write

- CRUD boilerplate (Get/List/Create/Update/Upsert/Delete implementations)
- RBAC checks (generated with correct verbs)
- MFA authorization (generated for mutating operations)
- Marshal/unmarshal helpers
- Read-only getter interface (`ThingsGetter`)
- Backend storage layer
- API client
- Auth `servicesGenerated` struct field + constructor wiring (`lib/auth/services.gen.go`)
- Cache `generatedConfig` embedded interface (`lib/cache/index.gen.go`)
- Authclient `cacheGeneratedServices` getter embed (`lib/auth/authclient/api.gen.go`)
- Cache collection builder (data loading and watching)
- Cache accessor methods (Get/List on `*Cache`)
- Events parser
- Event proto messages in `events.proto` (OneOf entries + message definitions — auto-injected)
- Event type string constants (`lib/events/api.gen.go`)
- Event code constants (`lib/events/codes.gen.go`)
- Event dynamic factory registrations
- Event test map registrations
- Event OneOf converter registrations
- tctl handler (get/create/update/delete + text formatting)
- Web UI audit event metadata (event codes, formatters, icons, fixtures in `generatedResourceEvents.gen.ts`)
- init() registration wiring
