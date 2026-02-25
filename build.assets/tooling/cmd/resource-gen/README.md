# resource-gen

Proto-driven code generator for Teleport resource services. Reads gRPC service
definitions annotated with a `resource_config` option and generates CRUD
boilerplate: service interfaces, backend storage, gRPC handlers, API client
methods, cache wiring, tctl handlers, and registration glue.

## Usage

```bash
cd build.assets/tooling && go run ./cmd/resource-gen \
  --proto-dir=../../api/proto \
  --output-dir=../.. \
  --module=github.com/gravitational/teleport
```

| Flag | Default | Description |
|------|---------|-------------|
| `--proto-dir` | (required) | Directory to scan recursively for `.proto` files containing `resource_config` |
| `--output-dir` | `.` | Root directory for generated file output |
| `--module` | (required unless `--events-only`) | Go module path (e.g. `github.com/gravitational/teleport`) |
| `--dry-run` | `false` | Print planned file paths without writing anything |
| `--events-only` | `false` | Only inject event messages into `events.proto` (no Go file generation) |

The tool is invoked automatically by `make grpc/host` in a two-phase pipeline:
first with `--events-only` to inject event messages into `events.proto`, then
after proto compilation to generate Go files.

## How it works

1. **Parse** — scans `--proto-dir` for `.proto` files containing `resource_config`
   service options. Extracts the resource kind, storage pattern, operations
   (Get/List/Create/Update/Upsert/Delete), and configuration.
2. **Validate** — checks spec consistency (e.g. Upsert requires Create, List
   requires Get, singleton storage forbids List, cache requires List).
3. **Generate** — renders Go templates for each resource, plus cross-resource
   files that aggregate all resources into shared types.
4. **Write** — writes generated files. Scaffold files are created once and never
   overwritten; `.gen.go` files are always overwritten.

## Proto option reference

Annotate a gRPC service with `teleport.options.v1.resource_config`:

```protobuf
import "teleport/options/v1/resource.proto";

service ThingService {
  option (teleport.options.v1.resource_config) = {
    storage: {
      backend_prefix: "things"
      standard: {}
    }
  };

  rpc GetThing(GetThingRequest) returns (Thing);
  rpc ListThings(ListThingsRequest) returns (ListThingsResponse);
  rpc CreateThing(CreateThingRequest) returns (Thing);
  rpc UpdateThing(UpdateThingRequest) returns (Thing);
  rpc DeleteThing(DeleteThingRequest) returns (google.protobuf.Empty);
}
```

Operations are inferred from RPC method names: `Get{Kind}`, `List{Plural}`,
`Create{Kind}`, `Update{Kind}`, `Upsert{Kind}`, `Delete{Kind}`.

### Configuration fields

All fields have sensible defaults. Only set values that differ from defaults.

#### `storage` (required)

| Field | Description |
|-------|-------------|
| `backend_prefix` | Backend key prefix (e.g. `"things"`) |
| `standard {}` | Flat layout: `prefix/<name>` |
| `singleton { fixed_name: "..." }` | Single resource: `prefix/<fixed_name>`. No List support. |
| `scoped { by: "username" }` | Scoped layout: `prefix/<scope>/<name>`. The `by` value is a proto field name on request messages. |

Exactly one of `standard`, `singleton`, or `scoped` must be set.

#### `cache`

| Field | Default | Description |
|-------|---------|-------------|
| `enabled` | `true` | Generate cache collection and accessor files |
| `indexes` | `["metadata.name"]` | Indexed field paths for cache lookups |

#### `pagination`

| Field | Default | Description |
|-------|---------|-------------|
| `default_page_size` | `200` | Used when client sends 0 |
| `max_page_size` | `1000` | Clamped in backend List |

#### `audit`

| Field | Default | Description |
|-------|---------|-------------|
| `emit_on_create` | `true` | Emit audit event on Create |
| `emit_on_update` | `true` | Emit audit event on Update/Upsert |
| `emit_on_delete` | `true` | Emit audit event on Delete |
| `emit_on_get` | `false` | Emit audit event on Get/List |
| `code_prefix` | — | Prefix for event code constants. 2-4 uppercase ASCII characters (e.g., `"CK"`). Required when any `emit_on_*` is true. |

#### `tctl`

| Field | Default | Description |
|-------|---------|-------------|
| `description` | `"<Kind> resources"` | Shown in tctl help |
| `mfa_required` | `true` | Mutation commands require MFA |
| `columns` | `["metadata.name"]` | Default `tctl get` text columns |
| `verbose_columns` | `["metadata.name", "metadata.revision", "metadata.expires"]` | Columns with `--verbose` |

#### `hooks`

| Field | Default | Description |
|-------|---------|-------------|
| `enable_lifecycle_hooks` | `false` | Generate `Hooks` struct with Before/After callbacks |

## Generated files

### Per-resource files

For a resource kind `thing` in package `teleport.thing.v1`:

| File | Description | Overwritten? |
|------|-------------|-------------|
| `lib/services/thing.gen.go` | `Things` + `ThingsGetter` interfaces, marshal/unmarshal | Always |
| `lib/services/local/thing.gen.go` | Backend using `generic.ServiceWrapper` | Always |
| `lib/auth/thing/thingv1/service.gen.go` | gRPC handler with RBAC + MFA | Always |
| `api/client/thing.gen.go` | API client methods | Always |
| `lib/auth/thing_register.gen.go` | Auth server init() wiring | Always |
| `lib/services/local/thing_register.gen.go` | Event parser init() wiring | Always |
| `lib/cache/thing_register.gen.go` | Cache collection init() wiring | Always (if cache enabled) |
| `lib/cache/thing.gen.go` | Cache Get/List accessors | Always (if cache enabled) |
| `tool/tctl/common/resources/thing_register.gen.go` | tctl handler init() wiring | Always |
| `lib/services/thing.go` | `ValidateThing()` stub | **Scaffold** |
| `lib/services/thing_test.go` | Validation test skeleton | **Scaffold** |
| `lib/auth/thing/thingv1/service.go` | `ServiceConfig`, authorize, audit events | **Scaffold** |

Scaffold files are created once. They contain stubs that the developer fills in
(validation logic, audit event types, custom authorization). `resource-gen` will
never overwrite them.

### Cross-resource files

These aggregate all resources into shared types. They are regenerated in full on
every run.

| File | Description |
|------|-------------|
| `api/types/constants.gen.go` | `Kind*` constants for all resources |
| `lib/auth/services.gen.go` | `servicesGenerated` struct + constructor |
| `lib/cache/index.gen.go` | `generatedConfig` struct (cache-enabled resources only) |
| `lib/auth/authclient/api.gen.go` | `cacheGeneratedServices` interface |

### Cross-resource event files

These aggregate event registrations across all resources that have audit events
enabled. They are regenerated in full on every run.

| File | Description |
|------|-------------|
| `lib/events/api.gen.go` | Event type string constants (e.g., `CookieCreateEvent = "resource.cookie.create"`) |
| `lib/events/codes.gen.go` | Event code constants (e.g., `CookieCreateCode = "CK001I"`) |
| `lib/events/dynamic.gen.go` | Dynamic factory `init()` registrations |
| `lib/events/events_test.gen.go` | Test event map `init()` registrations |
| `api/types/events/oneof.gen.go` | OneOf converter `init()` registrations |

### Automatic event proto injection

When `--events-only` is passed, `resource-gen` automatically injects event
messages into `api/proto/teleport/legacy/types/events/events.proto`. For each
resource with audit events enabled, it:

1. Compiles `events.proto` using `buf export` + `protocompile` to discover
   existing OneOf entries and the max field number.
2. Determines which event messages are missing (e.g. `CookieCreate`,
   `CookieUpdate`, `CookieDelete`).
3. Inserts new OneOf entries with sequential field numbers.
4. Appends message definitions at the end of the file.

This is append-only — existing entries are never removed. The injection is
idempotent and safe to run multiple times.

## Adding a new resource

See [`docs/resource-codegen/new-resource.md`](../../../../docs/resource-codegen/new-resource.md)
for the step-by-step guide.

## Architecture

```
cmd/resource-gen/
  main.go                       CLI entry point, file writing
  parser/                       Proto file scanner and option extractor
  spec/                         ResourceSpec types and validation
  generators/
    registry.go                 Generator registry (all per-resource generators)
    render.go                   Template rendering helpers
    helpers.go                  Name manipulation (pluralize, exportedName, etc.)
    events_proto_injection.go   Proto-aware event message injection (--events-only)
    templates/*.tmpl            Go templates for each generated file
    *_gathering.go              Cross-resource generators
```

The parser produces `[]spec.ResourceSpec`. Each spec is validated, then passed
through the generator registry. Cross-resource generators receive the full slice
and produce aggregated output files. All results are sorted by kind name for
deterministic output.
