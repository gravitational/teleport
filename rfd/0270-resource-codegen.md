---
authors: tener
state: implemented
---

# RFD 270 - Resource Code Generation

## Required Approvers

- Engineering: @rosstimothy (TBD)

## What

Code generation system for Teleport resources driven by protobuf options. Generates backend services, gRPC handlers, API clients, tctl integration, and cache layer from proto service definitions with configuration annotations.

## Why

Currently, adding a new resource requires manually creating 12 files with significant boilerplate:
- Backend service interface and implementation
- gRPC service handler with authorization
- API client methods
- tctl command handlers and formatters
- Cache collection (if cached)
- Event parser registration
- Wire-up in multiple locations

This boilerplate is ~40-50% mechanical and follows established patterns, yet must be written by hand. This leads to:
- Slow feature development
- Inconsistent implementations
- Errors in boilerplate (missing authorization checks, incorrect audit events)
- Difficult maintenance

## Details

### Proto Options Design

Resources are configured via service-level proto options that capture all necessary decisions. **Operations are determined by which RPCs you define** - the generator detects and implements them automatically.

**Only storage configuration is required.** All other options have reasonable defaults.

#### Minimal Configuration

```protobuf
syntax = "proto3";

package teleport.foo.v1;

import "teleport/options/v1/resource.proto";
import "teleport/foo/v1/foo.proto";

service FooService {
  option (teleport.options.v1.resource_config) = {
    // REQUIRED: Storage configuration
    storage {
      backend_prefix: "foo"
      standard {}  // or singleton{} or scoped{}
    }

    // Everything else is optional with defaults
  };

  // Define the RPCs you want
  rpc GetFoo(GetFooRequest) returns (Foo);
  rpc ListFoos(ListFoosRequest) returns (ListFoosResponse);
  rpc CreateFoo(CreateFooRequest) returns (Foo);
  rpc UpdateFoo(UpdateFooRequest) returns (Foo);
  rpc DeleteFoo(DeleteFooRequest) returns (google.protobuf.Empty);
}
```

#### Full Configuration (with defaults shown)

```protobuf
service FooService {
  option (teleport.options.v1.resource_config) = {
    // REQUIRED: Storage configuration
    storage {
      backend_prefix: "foo"
      standard {}
    }

    // OPTIONAL: Cache configuration (defaults shown)
    cache {
      enabled: true                    // Default: true
    }
    // Note: cache.indexes and cache.load_secrets are defined in the schema
    // but not yet configurable — the generator rejects non-default values.

    // OPTIONAL: tctl presentation (defaults shown)
    tctl {
      description: "Foo resources"     // Default: "<Kind> resources" (CamelCase kind name)
      mfa_required: true               // Default: true (secure by default)
      columns: ["metadata.name"]       // Default: ["metadata.name"]
      verbose_columns: [               // Default shown below
        "metadata.name",
        "metadata.revision",
        "metadata.expires"
      ]
    }

    // OPTIONAL: Audit events (defaults shown)
    audit {
      emit_on_create: true             // Default: true
      emit_on_update: true             // Default: true
      emit_on_delete: true             // Default: true
      // emit_on_get is defined in the schema but not yet supported —
      // the generator rejects true values. Add read-audit manually if needed.
    }

    // OPTIONAL: Extension points (defaults shown)
    hooks {
      enable_lifecycle_hooks: false    // Default: false
    }

    // OPTIONAL: Operations behavior (defaults shown)
    // Note: maps to BehaviorConfig in the generator's internal spec.
    operations {
      immutable: false                 // Default: false
    }

    // OPTIONAL: Pagination (defaults shown)
    pagination {
      default_page_size: 200           // Default: 200
      max_page_size: 1000              // Default: 1000
    }
  };

  rpc GetFoo(GetFooRequest) returns (Foo);
  rpc ListFoos(ListFoosRequest) returns (ListFoosResponse);
  rpc CreateFoo(CreateFooRequest) returns (Foo);
  rpc UpdateFoo(UpdateFooRequest) returns (Foo);
  rpc DeleteFoo(DeleteFooRequest) returns (google.protobuf.Empty);
}
```

### Option Schema Definition

```protobuf
// In api/proto/teleport/options/v1/resource.proto
syntax = "proto3";

package teleport.options.v1;

import "google/protobuf/descriptor.proto";

extend google.protobuf.ServiceOptions {
  ResourceConfig resource_config = 50001;
}

message ResourceConfig {
  StorageConfig storage = 1;
  CacheConfig cache = 2;
  TctlConfig tctl = 3;
  AuditConfig audit = 4;
  HooksConfig hooks = 5;
  OperationsConfig operations = 6;
  PaginationConfig pagination = 7;
}

message StorageConfig {
  // Backend key prefix (e.g., "foo")
  string backend_prefix = 1;

  // Storage pattern (determines key structure)
  oneof pattern {
    StandardStorage standard = 2;
    SingletonStorage singleton = 3;
    ScopedStorage scoped = 4;
  }
}

message StandardStorage {
  // Flat storage at backend_prefix/<name>
  // Keys: "foo/my-resource"
}

message SingletonStorage {
  // Single resource at backend_prefix/<fixed_name>
  // Keys: "foo/cluster"
  string fixed_name = 1;
}

message ScopedStorage {
  // Hierarchical storage at backend_prefix/<scope>/<name>
  // Keys: "foo/alice/challenge-123"
  string by = 1;  // Scope parameter name (e.g., "username", "tenant_id")
}

message CacheConfig {
  optional bool enabled = 1;           // Default: true
  repeated string indexes = 2;         // Default: ["metadata.name"]
  bool load_secrets = 3;               // Default: false
}

message TctlConfig {
  string description = 1;              // Default: "<Kind> resources"
  optional bool mfa_required = 2;      // Default: true (secure by default)
  repeated string columns = 3;         // Default: ["metadata.name"]
  repeated string verbose_columns = 4; // Default: ["metadata.name", "metadata.revision", "metadata.expires"]
}

message AuditConfig {
  optional bool emit_on_create = 1;  // Default: true
  optional bool emit_on_update = 2;  // Default: true
  optional bool emit_on_delete = 3;  // Default: true
  optional bool emit_on_get = 4;     // Default: false (not yet supported — generator rejects true)
}

message HooksConfig {
  bool enable_lifecycle_hooks = 1;  // Default: false
}

message OperationsConfig {
  // Behavior modifiers (NOT which operations exist - that's determined by RPCs)
  // Prevent updates/upserts after creation (generated handlers return error)
  bool immutable = 1;  // Default: false
}

message PaginationConfig {
  optional int32 default_page_size = 1;  // Default: 200
  optional int32 max_page_size = 2;      // Default: 1000
}
```

### Configuration Defaults

**Required configuration:**
- `storage.backend_prefix` - no default, must be specified
- `storage.pattern` - must choose one: `standard`, `singleton`, or `scoped`

**All other options have defaults:**

| Option | Default Value | Rationale |
|--------|---------------|-----------|
| **Cache** | | |
| `cache.enabled` | `true` | Most resources benefit from caching |
| `cache.indexes` | `["metadata.name"]` | Name is always indexed. *Custom values not yet supported — generator rejects non-default values.* |
| `cache.load_secrets` | `false` | Secure by default - secrets not cached. *Not yet supported — generator rejects `true`.* |
| **tctl** | | |
| `tctl.description` | `"<Kind> resources"` | Inferred from CamelCase proto message name |
| `tctl.mfa_required` | `true` | Secure by default - require MFA for mutations |
| `tctl.columns` | `["metadata.name"]` | Minimal useful default |
| `tctl.verbose_columns` | `["metadata.name", "metadata.revision", "metadata.expires"]` | Show all metadata in verbose mode |
| **Audit** | | |
| `audit.emit_on_create` | `true` | All mutations audited by default |
| `audit.emit_on_update` | `true` | All mutations audited by default |
| `audit.emit_on_delete` | `true` | All mutations audited by default |
| `audit.emit_on_get` | `false` | Read operations not audited (too noisy). *Not yet supported — generator rejects `true`.* |
| **Hooks** | | |
| `hooks.enable_lifecycle_hooks` | `false` | No overhead unless needed |
| **Operations** | | |
| `operations.immutable` | `false` | Most resources are mutable |
| **Pagination** | | |
| `pagination.default_page_size` | `200` | Balance between performance and UX |
| `pagination.max_page_size` | `1000` | Prevent excessive memory usage |

### Operations Detection

**Operations are automatically detected from RPCs defined in the service.** No configuration needed.

| RPC Defined | Implementation Generated |
|-------------|--------------------------|
| `GetFoo` | Backend Get + gRPC handler + API client method |
| `ListFoos` | Backend List + gRPC handler + API client method |
| `CreateFoo` | Backend Create + gRPC handler + API client method |
| `UpdateFoo` | Backend Update + gRPC handler + API client method |
| `UpsertFoo` | Backend Upsert + gRPC handler + API client method |
| `DeleteFoo` | Backend Delete + gRPC handler + API client method |

**Constraints** (enforced by the parser during proto processing):
- **List is not supported for singleton storage** — the parser returns an error if a List RPC is defined for a singleton service.
- **Upsert requires Create** — if `UpsertFoo` is defined, `CreateFoo` must also be defined (upsert audit delegates to create).
- **Pagination bounds** — `default_page_size` and `max_page_size` must both be > 0, and `max_page_size` must be >= `default_page_size`.

**Note on Upsert:** Per [RFD 153](./0153-resource-guidelines.md), `Create` and `Update` should be preferred over `Upsert` for normal operations. Upsert uses unconditional `Put` semantics (no revision check), making it unsuitable as a general-purpose write operation. Only define `UpsertFoo` if you have a specific need for create-or-replace semantics.

To omit an operation, simply don't define the RPC. For example, immutable resources (certificates, audit events) should not have `UpdateFoo` or `UpsertFoo` RPCs.

### Storage Patterns

#### Standard (Flat) Storage

Default pattern for most resources. Stores at `prefix/<name>`.

**Minimal configuration (uses all defaults):**

```protobuf
service RoleService {
  option (teleport.options.v1.resource_config) = {
    storage {
      backend_prefix: "roles"
      standard {}
    }
    // All other config uses defaults:
    // - cache: enabled with ["metadata.name"] index
    // - tctl: "Role resources", MFA required
    // - audit: emit on create/update/delete
    // - pagination: 200/1000
  };

  rpc GetRole(GetRoleRequest) returns (Role);
  rpc ListRoles(ListRolesRequest) returns (ListRolesResponse);
  rpc CreateRole(CreateRoleRequest) returns (Role);
  rpc UpdateRole(UpdateRoleRequest) returns (Role);
  rpc DeleteRole(DeleteRoleRequest) returns (google.protobuf.Empty);
}

message GetRoleRequest {
  string name = 1;  // Generator expects 'name' field
}
```

**Override defaults as needed:**

```protobuf
service RoleService {
  option (teleport.options.v1.resource_config) = {
    storage {
      backend_prefix: "roles"
      standard {}
    }

    // Customize tctl display
    tctl {
      columns: ["metadata.name", "spec.allow.logins"]
    }
  };
  // ...
}
```

Generated storage keys: `roles/admin`, `roles/developer`, etc.

#### Singleton Storage

Single resource instance. Stores at `prefix/<fixed_name>`.

```protobuf
service ClusterConfigService {
  option (teleport.options.v1.resource_config) = {
    storage {
      backend_prefix: "cluster_config"
      singleton {
        fixed_name: "cluster"
      }
    }

    tctl {
      description: "Cluster configuration"
      columns: ["metadata.name"]
    }
  };

  // Only Get and Update shown here — List is rejected for singletons;
  // Create and Delete are also valid.
  rpc GetClusterConfig(GetClusterConfigRequest) returns (ClusterConfig);
  rpc UpdateClusterConfig(UpdateClusterConfigRequest) returns (ClusterConfig);
}

message GetClusterConfigRequest {
  // No 'name' field - generator uses fixed_name from config
}
```

Generated storage key: `cluster_config/cluster`

#### Scoped Storage

Hierarchical storage per scope. Stores at `prefix/<scope>/<name>`.

```protobuf
service MFAChallengeService {
  option (teleport.options.v1.resource_config) = {
    storage {
      backend_prefix: "mfa_challenge"
      scoped {
        by: "username"
      }
    }
  };

  rpc CreateMFAChallenge(CreateMFAChallengeRequest) returns (MFAChallenge);
  rpc GetMFAChallenge(GetMFAChallengeRequest) returns (MFAChallenge);
  rpc ListMFAChallenges(ListMFAChallengesRequest) returns (ListMFAChallengesResponse);
  rpc DeleteMFAChallenge(DeleteMFAChallengeRequest) returns (google.protobuf.Empty);
}

message GetMFAChallengeRequest {
  string username = 1;  // Generator requires both scope field and name
  string name = 2;
}

message ListMFAChallengesRequest {
  string username = 1;  // List within scope
  int32 page_size = 2;
  string page_token = 3;
}

message DeleteMFAChallengeRequest {
  string username = 1;  // Scope field required
  string name = 2;
}
```

Generated storage keys: `mfa_challenge/alice/challenge-1`, `mfa_challenge/bob/challenge-2`, etc.

#### Immutable Resources

Some resources cannot be modified after creation (certificates, tokens, audit events). Use `operations.immutable` to document this:

```protobuf
service CertificateService {
  option (teleport.options.v1.resource_config) = {
    storage {
      backend_prefix: "cert"
      standard {}
    }

    operations {
      immutable: true  // Documents that certificates cannot be updated
    }

    tctl {
      description: "X.509 certificates"
      mfa_required: false
      columns: ["metadata.name", "metadata.expires"]
    }

    audit {
      emit_on_create: true
      emit_on_delete: true
    }
  };

  rpc GetCertificate(GetCertificateRequest) returns (Certificate);
  rpc CreateCertificate(CreateCertificateRequest) returns (Certificate);
  rpc DeleteCertificate(DeleteCertificateRequest) returns (google.protobuf.Empty);

  // No UpdateCertificate or UpsertCertificate RPCs - immutable resources
}
```

**Note:** If someone later adds `UpdateCertificate` RPC by mistake, the `immutable: true` flag will cause the generated implementation to return an error. This serves as documentation and protection against accidental modifications.

### Generated Code

From a proto service with `resource_config` options, the generator produces up to 12 files per resource (7 always-generated `.gen.go` files + 2 cache `.gen.go` files when `cache.enabled` is true + 3 scaffold `.go` files). Rather than modifying shared files like `grpcserver.go` or `client.go`, the generator creates standalone `_register.gen.go` files that use `init()` functions for registration.

```
Input:
  api/proto/teleport/foo/v1/foo_service.proto (with options)

Output (generated - fully overwritten on each run):
  ✅ lib/services/foo.gen.go
     - Service interfaces (Foos, FoosGetter)
     - Marshal/Unmarshal functions

  ✅ lib/services/local/foo.gen.go
     - Backend implementation using generic.ServiceWrapper
     - Pagination enforcement (default_page_size, max_page_size)

  ✅ lib/auth/foo/foov1/service.gen.go
     - gRPC service implementation
     - Reader interface (Get/List — the type used by ServiceConfig.Reader)
     - Authorization checks (RBAC)
     - Backend delegation
     - Hook callsites (if hooks.enable_lifecycle_hooks)
     - Audit event emission (if audit.emit_on_create/update/delete)
     - eventStatus() and getExpires() helpers (when any audit flag is true)

  ✅ api/client/foo.gen.go
     - Client methods for each RPC

  ✅ lib/auth/foo_register.gen.go
     - Auth gRPC wiring via init()

  ✅ lib/cache/foo_register.gen.go (if cache.enabled)
     - Cache collection wiring via init()
     - Compile-time static assertion and index type

  ✅ lib/cache/foo.gen.go (if cache.enabled)
     - Cache accessor methods (Get/List on *Cache)
     - FooCollection() method

  ✅ lib/services/local/foo_register.gen.go
     - Events parser wiring via init()

  ✅ tool/tctl/common/resources/foo_register.gen.go
     - tctl handler wiring via init()
     - Resource formatter (columns from tctl.columns config)

Output (scaffold - created once, never overwritten):
  ✅ lib/services/foo_custom.go
     - ValidateFoo() stub (exported, follows RFD 153 convention)

  ✅ lib/services/foo_custom_test.go
     - Test skeleton for ValidateFoo()

  ✅ lib/auth/foo/foov1/service_custom.go
     - ServiceConfig, Service struct, NewService() constructor
     - authorize() and authorizeMutation() methods
     - Fully-populated audit event implementations with FIXME markers
```

### Extension Points

Not all code can be generated. The following extension points allow custom logic via scaffold files that are created once and never overwritten by the generator.

#### 1. Validation Function

The generator creates a scaffold file at `lib/services/foo_custom.go` with an exported `ValidateFoo()` stub, following the established convention from RFD 153 (see `ValidateWorkloadIdentity`, `ValidateHealthCheckConfig`, `ValidateSPIFFEFederation`, etc. in `lib/services/`). Implement validation logic there:

```go
// Scaffold: lib/services/foo_custom.go (created once, never overwritten)
func ValidateFoo(f *foov1.Foo) error {
    if f.Spec.Bar == "" {
        return trace.BadParameter("bar is required")
    }
    // Custom validation logic...
    return nil
}

// Test scaffold: lib/services/foo_custom_test.go
func TestValidateFoo(t *testing.T) {
    // Add validation test cases here
}
```

The generated gRPC service (`service.gen.go`) calls `services.ValidateFoo()` in Create and Update handlers. Because validation is exported and lives in `lib/services/`, it is reusable across callers (gRPC service, tctl, tests, etc.).

#### 2. Service Scaffold (ServiceConfig, Auth, and Audit Events)

The gRPC service scaffold file (`lib/auth/foo/foov1/service_custom.go`) is the primary developer-owned file. It contains the service struct definitions, authorization methods, and fully-populated audit event implementations. The generated `service.gen.go` references types and methods defined here — if they are missing, the Go compiler will produce an error (convention contract).

The scaffold contains:

- **`ServiceConfig` struct** — standard fields (`Authorizer`, `Backend`, `Reader`, `Emitter`) plus `*Hooks` when `hooks.enable_lifecycle_hooks` is true, plus `// TODO` markers for resource-specific dependencies. `Reader` is a generated interface (in `service.gen.go`) covering read operations (`Get`, `List`), wired to the cache layer at runtime.
- **`CheckAndSetDefaults()`** — validates required config fields.
- **`Service` struct** — embeds `UnimplementedFooServiceServer`, standard fields, plus `// TODO` markers for resource-specific fields.
- **`NewService(cfg ServiceConfig)`** — constructor that wires config into the service.
- **`authorize(ctx, verbs...)`** — read authorization. Override to add module gating, custom RBAC, etc.
- **`authorizeMutation(ctx, verbs...)`** — write authorization (delegates to `authorize()`, adds MFA). Override for custom write-path checks.
- **`emitCreateAuditEvent()`**, **`emitUpdateAuditEvent()`**, **`emitDeleteAuditEvent()`**, and **`emitUpsertAuditEvent()`** (when Upsert is defined) — fully-populated audit event implementations with `FIXME` markers for resource-specific fields. The upsert method delegates to `emitCreateAuditEvent` when the old resource is nil (new create) or `emitUpdateAuditEvent` otherwise. Note: `emitUpsertAuditEvent` is gated on the `audit.emit_on_update` flag (not a separate flag).

```go
// Scaffold: lib/auth/foo/foov1/service_custom.go (created once, never overwritten)

type ServiceConfig struct {
    Authorizer authz.Authorizer
    Backend    services.Foos
    Reader     Reader
    Emitter    apievents.Emitter
    // TODO: add resource-specific dependencies
}

func (s *ServiceConfig) CheckAndSetDefaults() error { /* ... */ }

type Service struct {
    foov1pb.UnimplementedFooServiceServer
    authorizer authz.Authorizer
    backend    services.Foos
    reader     Reader
    emitter    apievents.Emitter
    // TODO: add resource-specific fields
}

func NewService(cfg ServiceConfig) (*Service, error) { /* ... */ }

// authorize checks read authorization for the given verbs.
func (s *Service) authorize(ctx context.Context, verbs ...string) (*authz.Context, error) {
    authCtx, err := s.authorizer.Authorize(ctx)
    if err != nil {
        return nil, trace.Wrap(err)
    }
    if err := authCtx.CheckAccessToKind(resourceKind, verbs...); err != nil {
        return nil, trace.Wrap(err)
    }
    return authCtx, nil
}

// authorizeMutation checks write authorization with MFA.
func (s *Service) authorizeMutation(ctx context.Context, verbs ...string) (*authz.Context, error) {
    authCtx, err := s.authorize(ctx, verbs...)
    if err != nil {
        return nil, trace.Wrap(err)
    }
    if err := authCtx.AuthorizeAdminActionAllowReusedMFA(); err != nil {
        return nil, trace.Wrap(err)
    }
    return authCtx, nil
}

// Audit event implementations follow the CrownJewel/UserTask pattern:
// - Emit before error check (records both success and failure)
// - Update/Upsert fetch old resource for diff in audit event
// - FIXME markers for resource-specific event fields

func (s *Service) emitCreateAuditEvent(ctx context.Context, foo *foov1pb.Foo, authCtx *authz.Context, err error) {
    if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.FooCreate{
        Metadata: apievents.Metadata{
            Type: libevents.FooCreateEvent,
            Code: libevents.FooCreateCode,
        },
        UserMetadata:       authCtx.GetUserMetadata(),
        ConnectionMetadata: authz.ConnectionMetadata(ctx),
        Status:             eventStatus(err),
        ResourceMetadata: apievents.ResourceMetadata{
            Name:      foo.GetMetadata().GetName(),
            Expires:   getExpires(foo.GetMetadata().GetExpires()),
            UpdatedBy: authCtx.Identity.GetIdentity().Username,
        },
        // FIXME: add resource-specific event fields here.
    }); auditErr != nil {
        slog.WarnContext(ctx, "Failed to emit foo create event.", "error", auditErr)
    }
}

func (s *Service) emitUpdateAuditEvent(ctx context.Context, old, new *foov1pb.Foo, authCtx *authz.Context, err error) { /* same pattern, includes old resource for diff */ }
func (s *Service) emitDeleteAuditEvent(ctx context.Context, name string, authCtx *authz.Context, err error) { /* same pattern, uses name and UpdatedBy — no Expires field */ }

// Generated when UpsertFoo RPC is defined (gated on audit.emit_on_update):
func (s *Service) emitUpsertAuditEvent(ctx context.Context, old, new *foov1pb.Foo, authCtx *authz.Context, err error) {
    if old == nil {
        s.emitCreateAuditEvent(ctx, new, authCtx, err)
        return
    }
    s.emitUpdateAuditEvent(ctx, old, new, authCtx, err)
}
```

The audit methods reference event types and constants that must be defined by the developer:

1. **Event types** — `apievents.FooCreate`, `apievents.FooUpdate`, `apievents.FooDelete` structs (see `CrownJewelCreate` for the pattern).
2. **Event type constants** — `libevents.FooCreateEvent`, `libevents.FooUpdateEvent`, `libevents.FooDeleteEvent` in `lib/events/codes.go`.
3. **Event code constants** — `libevents.FooCreateCode`, `libevents.FooUpdateCode`, `libevents.FooDeleteCode` in `lib/events/codes.go`.

The generated `service.gen.go` calls `s.authorize()` for read operations and `s.authorizeMutation()` for write operations, then delegates to the scaffold's audit emit methods. The `eventStatus()` and `getExpires()` helpers are generated in `service.gen.go` (only when at least one audit flag is true).

#### 3. Lifecycle Hooks

When `hooks.enable_lifecycle_hooks: true` is set, the generator produces a `Hooks` struct in `service.gen.go` with optional function fields for each operation. The scaffold's `ServiceConfig` and `Service` gain a `*Hooks` field, and the generated CRUD methods include nil-checked callsites:

```go
// Generated: lib/auth/foo/foov1/service.gen.go

// Hooks defines optional lifecycle callbacks for Foo operations.
// Each Before/After pair is only generated when the corresponding RPC is defined.
type Hooks struct {
    BeforeCreate func(context.Context, *foov1pb.Foo) error   // Only when CreateFoo RPC exists
    AfterCreate  func(context.Context, *foov1pb.Foo)          // Only when CreateFoo RPC exists
    BeforeUpdate func(context.Context, *foov1pb.Foo) error   // Only when UpdateFoo RPC exists
    AfterUpdate  func(context.Context, *foov1pb.Foo)          // Only when UpdateFoo RPC exists
    BeforeUpsert func(context.Context, *foov1pb.Foo) error   // Only when UpsertFoo RPC exists
    AfterUpsert  func(context.Context, *foov1pb.Foo)          // Only when UpsertFoo RPC exists
    BeforeDelete func(context.Context, string) error          // Only when DeleteFoo RPC exists
    AfterDelete  func(context.Context, string)                 // Only when DeleteFoo RPC exists
}

func (s *Service) CreateFoo(ctx context.Context, req *foov1pb.CreateFooRequest) (*foov1pb.Foo, error) {
    authCtx, err := s.authorizeMutation(ctx, types.VerbCreate)
    if err != nil {
        return nil, trace.Wrap(err)
    }
    if err := services.ValidateFoo(req.GetFoo()); err != nil {
        return nil, trace.Wrap(err)
    }
    // Before hook (nil-checked)
    if s.hooks != nil && s.hooks.BeforeCreate != nil {
        if err := s.hooks.BeforeCreate(ctx, req.GetFoo()); err != nil {
            return nil, trace.Wrap(err)
        }
    }
    rsp, err := s.backend.CreateFoo(ctx, req.GetFoo())
    s.emitCreateAuditEvent(ctx, rsp, authCtx, err) // Only emitted when audit.emit_on_create is true
    if err != nil {
        return nil, trace.Wrap(err)
    }
    // After hook (nil-checked, only on success)
    if s.hooks != nil && s.hooks.AfterCreate != nil {
        s.hooks.AfterCreate(ctx, rsp)
    }
    return rsp, nil
}

// Scaffold: lib/auth/foo/foov1/service_custom.go gains:
//   ServiceConfig.Hooks *Hooks
//   Service.hooks       *Hooks
//   NewService wires cfg.Hooks into s.hooks
```

Callers provide hooks by setting function fields on the `Hooks` struct when constructing the service. Unused hooks are simply left nil.

#### 4. Immutable Resources

When `operations.immutable: true` is set, the generator produces backend methods that reject mutations. The guard lives in the **backend implementation** (`lib/services/local/`), so any caller (gRPC, tests, CLI) that attempts an update receives the error:

```go
// Generated: lib/services/local/certificate.gen.go (backend layer)

// If UpdateCertificate RPC exists despite immutable flag:
func (s *CertificateService) UpdateCertificate(_ context.Context, _ *certificatev1.Certificate) (*certificatev1.Certificate, error) {
    return nil, trace.BadParameter("certificates are immutable and cannot be updated")  // uses "<lower>s" suffix
}

// If UpsertCertificate RPC exists despite immutable flag:
func (s *CertificateService) UpsertCertificate(_ context.Context, _ *certificatev1.Certificate) (*certificatev1.Certificate, error) {
    return nil, trace.BadParameter("certificates are immutable and cannot be upserted")  // uses "<lower>s" suffix
}
```

**Best practice:** Don't define Update/Upsert RPCs for immutable resources. The `immutable` flag serves as documentation and protection against accidental additions.

### Generator Implementation

The generator runs as part of `make grpc/host` (local) or `make grpc` (inside buildbox container):

```makefile
# From the repo-root Makefile:
.PHONY: grpc/host
grpc/host: protos/all
	@build.assets/genproto.sh
	@$(MAKE) generate-resource-services/host

.PHONY: generate-resource-services/host
generate-resource-services/host:
	@cd build.assets/tooling && go run ./cmd/resource-gen \
		--proto-dir=../../api/proto \
		--output-dir=../.. \
		--module=github.com/gravitational/teleport
```

Generator workflow:

1. Parse proto files with `resource_config` options
2. Apply defaults for optional configuration
3. Detect operations from defined RPCs
4. Validate request shapes (storage pattern matches RPC signatures)
5. Validate spec constraints (`spec.Validate()` — pagination bounds, upsert-requires-create, not-yet-supported features)
6. Generate up to 9 `.gen.go` files (7 always, +2 when cache enabled; fully overwritten each run)
7. Generate 3 scaffold `.go` files (created once, never overwritten)
8. Registration files use `init()` functions to wire into shared infrastructure — no AST manipulation of existing files required

**Config-driven features implemented:**
- Pagination enforcement (`pagination.default_page_size`, `pagination.max_page_size`) in backend List
- Cache gating (`cache.enabled`) — cache files skipped entirely when false
- Audit flags (`audit.emit_on_create/update/delete`) control whether service calls audit stubs
- tctl columns from config (`tctl.columns`, `tctl.verbose_columns`)
- tctl MFA requirement (`tctl.mfa_required`)
- Lifecycle hooks (`hooks.enable_lifecycle_hooks`) — hook callsites and scaffold stubs
- Singleton `fixed_name` in backend Get/Delete (no name parameter needed)
- Immutable resources (`operations.immutable`) — Update/Upsert return `trace.BadParameter`
- Not-yet-supported features (`audit.emit_on_get`, custom `cache.indexes`, `cache.load_secrets`) — rejected with `trace.NotImplemented`

### Request Shape Validation

The parser validates that **Get, Delete, and List** request messages match the storage pattern (Create/Update/Upsert request shapes are not validated):

| Storage Pattern | Get Request Fields | Delete Request Fields | List Request Fields |
|----------------|-------------------|----------------------|---------------------|
| Standard | `string name` | `string name` | `int32 page_size`, `string page_token` |
| Singleton | `name` field must be absent | `name` field must be absent | (no List RPC) |
| Scoped | `string <scope_field>`, `string name` | `string <scope_field>`, `string name` | `string <scope_field>`, `int32 page_size`, `string page_token` |

Examples:

```protobuf
// Standard storage - valid
storage { standard {} }
message GetFooRequest {
  string name = 1;  // valid
}

// Singleton storage - valid (name field must be absent)
storage { singleton { fixed_name: "x" } }
message GetFooRequest {
  // No name field — valid (name field would be rejected)
}

// Scoped storage - valid
storage { scoped { by: "username" } }
message GetFooRequest {
  string username = 1;  // valid
  string name = 2;      // valid
}

// Scoped storage - INVALID
storage { scoped { by: "username" } }
message GetFooRequest {
  string name = 1;  // INVALID: missing username field
}
```

### Benefits

**For developers:**
- 12 files generated from one proto definition
- ~40-50% less code to write
- Decisions explicit in config file
- Fast prototyping (add proto + validation function = working resource)

**For maintainers:**
- Consistent patterns enforced by generator
- Easier to update (change generator, regenerate all resources)
- Less code to review (generated code is trusted)
- Centralized authorization/audit/error handling

**For security:**
- Authorization checks always present
- Audit events consistently emitted
- Input validation enforced
- Less opportunity for human error in boilerplate

### Adoption Strategy

**New Resources Only**

Codegen is for **new resources** going forward. Existing hand-written resources will remain as-is.

1. **Phase 1** (complete): Implement and validate generator
   - Generator built with all components (64 tests across 4 packages)
   - Validated through three friction log iterations (Widget, Sensor, Beacon)

2. **Phase 2** (current): Enable for new resources
   - First new resource uses codegen
   - Establish pattern and best practices
   - Update contributor guidelines

3. **Ongoing**: Standard practice
   - All new resources use codegen
   - Generator maintained and improved based on feedback
   - Existing resources unchanged unless refactored for other reasons

### Open Questions

1. **Multi-level scoping**: Support `prefix/<tenant>/<user>/<name>`?
   ```protobuf
   scoped {
     hierarchy: ["tenant_id", "username"]
   }
   ```

2. **Namespace support**: Built-in namespace handling?
   ```protobuf
   namespaced: true  // prefix/<namespace>/<name>
   ```

3. **Time-partitioned storage**: For audit events?
   ```protobuf
   partitioned {
     by_time: true
     granularity: DAY
   }
   ```

4. **Field-level options**: Cache indexes as field annotations?
   ```protobuf
   message Foo {
     string name = 1 [(teleport.cache_index) = true];
   }
   ```

**Note on custom authorization:** This does not require proto-level config. The scaffold's `authorize()` and `authorizeMutation()` methods can be edited directly to customize or replace the default authorization logic.

#### Out of Scope (Initial Implementation)

**Rejected by the generator** — these configurations are accepted by the proto schema but rejected at generation time with `trace.NotImplemented` errors:

- **`audit.emit_on_get`** — Read-audit support. Mutation audit (create/update/delete) is supported; read-audit can be added manually to the scaffold if needed.
- **`cache.indexes` beyond `["metadata.name"]`** — Custom cache indexes. Only the default `nameIndex` is generated. Additional indexes would require matching accessor methods.
- **`cache.load_secrets`** — Secret loading. Generated List RPCs do not have a secrets parameter. Resources with secrets should handle this manually (see the connector pattern in `lib/services/local/users.go`).

**Not implemented** — these features are not supported in the initial implementation:

- **Multi-level scoping** — Only single-level `scoped.by` is supported.
- **Namespace support** — Not implemented.
- **Time-partitioned storage** — Not implemented.
- **Custom marshaling detection** — The generator always generates standard proto marshaling. Custom marshal/unmarshal can be added manually.
- **`Range` iterator methods (`iter.Seq2`)** — Not generated. This is increasingly standard in Teleport (18+ implementations) and is planned for future codegen support (see Future Work). Currently, add Range methods manually.
- **`FieldMask` partial updates** — Not generated. Only a few resources in the codebase use FieldMask; it is not common for new resources. Can be added manually to the scaffold's Update handler if needed.

## Design Decisions

### Operations Detection vs Configuration

**Decision:** Operations (Get, List, Create, Update, Upsert, Delete) are **detected from RPCs defined** in the proto service, not configured via options.

**Alternatives considered:**

1. **Explicit operation config** (rejected)
   ```protobuf
   operations {
     enable_upsert: true  // Explicit opt-in
   }
   ```
   **Rejected:** Duplicates information already in proto. Config can drift from RPC definitions.

2. **Config drives, ignore proto** (rejected)
   ```protobuf
   operations {
     enable_upsert: true
   }
   // If UpsertFoo RPC exists but enable_upsert: false → ignore RPC
   ```
   **Rejected:** Confusing - why define RPC if it's ignored?

3. **Detect from RPCs** (chosen)
   ```protobuf
   // No config needed - just define the RPCs you want
   rpc UpsertFoo(UpsertFooRequest) returns (Foo);  // Presence = enabled
   ```
   **Chosen:** Single source of truth (proto), no duplication, clearer intent.

**Rationale:**
- Proto already defines the API contract - operations should be explicit there
- Avoids config/proto mismatches
- `operations` config reserved for behavior modifiers (e.g., immutable) not which operations exist

**Note on Upsert:** Per [RFD 153](./0153-resource-guidelines.md), `Create` and `Update` should be preferred over `Upsert`. Upsert uses unconditional `Put` semantics (no revision check), making it unsuitable as a general-purpose write operation. To discourage usage without preventing it, Upsert is only generated if the RPC is explicitly defined - there's no default or shorthand for it.

## Alternatives Considered

### Alternative 1: Go Struct Tags

Use Go struct tags to drive generation:

```go
type Foo struct {
    // ...

    //go:generate resource-gen --type=Foo --storage=standard --cache=true
}
```

**Rejected**: Proto is the source of truth. Duplicating config in Go breaks single source.

### Alternative 2: Separate YAML Config

Separate config file per resource:

```yaml
# foo_config.yaml
resource: Foo
storage:
  pattern: standard
  prefix: foo
cache:
  enabled: true
```

**Rejected**: Three files (proto, YAML, validation code) instead of two. Config can drift from proto.

### Alternative 3: Code Generation from Examples

Parse example resources and generate similar:

```bash
resource-gen --from=role.go --new=foo
```

**Rejected**: No explicit configuration. Decisions hidden in example. Hard to maintain.

## Summary: Configuration Options

**Minimal required configuration:**
```protobuf
storage {
  backend_prefix: "foo"
  standard {}  // or singleton{} or scoped{}
}
```

**Everything else has defaults.** Override only what differs from defaults.

The final schema includes **7 configuration sections**:

| Section | Required? | Options | Defaults |
|---------|-----------|---------|----------|
| **storage** | ✅ Yes | `backend_prefix`<br>`pattern`: standard/singleton/scoped<br>`singleton.fixed_name`<br>`scoped.by` | None - must specify |
| **cache** | ❌ No | `enabled`<br>`indexes` *(rejected if non-default)*<br>`load_secrets` *(rejected if true)* | `true`<br>`["metadata.name"]`<br>`false` |
| **tctl** | ❌ No | `description`<br>`mfa_required`<br>`columns`<br>`verbose_columns` | `"<Kind> resources"`<br>`true`<br>`["metadata.name"]`<br>`["metadata.name", "metadata.revision", "metadata.expires"]` |
| **audit** | ❌ No | `emit_on_create/update/delete`<br>`emit_on_get` *(rejected if true)* | `true/true/true`<br>`false` |
| **hooks** | ❌ No | `enable_lifecycle_hooks` | `false` |
| **operations** | ❌ No | `immutable` | `false` |
| **pagination** | ❌ No | `default_page_size`<br>`max_page_size` | `200`<br>`1000` |

**Operations (Get, List, Create, Update, Upsert, Delete)** are detected from RPCs defined - no configuration needed.

**Extension points** (scaffold files, created once, never overwritten):
- Validation: `lib/services/foo_custom.go` → `ValidateFoo()` (exported, initially returns `trace.NotImplemented`, follows RFD 153 convention)
- Validation tests: `lib/services/foo_custom_test.go`
- Service scaffold: `lib/auth/foo/foov1/service_custom.go` → `ServiceConfig`, `Service`, `NewService()`, `authorize()`, `authorizeMutation()`, fully-populated audit event implementations (`emitCreateAuditEvent`, `emitUpdateAuditEvent`, `emitDeleteAuditEvent`, and `emitUpsertAuditEvent` when Upsert is defined)
- Lifecycle hooks: generated in `service.gen.go` as `Hooks` struct with per-RPC conditional function fields (if `hooks.enable_lifecycle_hooks`)

## Security Considerations

- Generated authorization checks use standard patterns - consistent security posture
- Audit event callsites always generated for RPC-detected mutation operations — no missing callsites. Event payload correctness requires developer completion of FIXME markers in the scaffold. Scaffold code does not compile until audit event types are manually defined
- Generated code reviewed once in generator - reduces review surface
- Validation function remains manual - critical business logic still reviewed
- Hooks allow security-critical custom logic without forking generated code

## UX Considerations

**For resource implementers:**
- Faster feature development
- Less boilerplate to write
- Decisions explicit in one file
- Standard patterns enforced

**For tctl users:**
- Generated tctl integration matches hand-written when `tctl.columns` is configured; otherwise a fallback with `// CUSTOMIZE` marker is generated
- Consistent column formatting across resources
- MFA requirements explicit in config

**For API clients:**
- No change - generated client methods match hand-written
- Consistent error handling
- Standard pagination

## Future Work

1. **Range iterator generation (`iter.Seq2`)**: Generate `RangeFoos` methods returning `iter.Seq2[*foov1.Foo, error]` for backend and cache. This pattern is increasingly standard in Teleport (18+ implementations) and enables efficient streaming pagination. High priority.
2. **Secrets separation support**: Enable `cache.load_secrets` and generate conditional secret loading logic for resources with embedded secrets. Medium priority — most new resources don't have secrets.
3. **Web UI generation**: Generate basic CRUD UI components from proto + config
4. **OpenAPI generation**: Generate OpenAPI/Swagger from proto + config
5. **Resource documentation**: Generate docs from proto comments + config
6. **Migration tooling**: Detect hand-written resources that match generated patterns, offer to migrate
7. **Validation generation**: Generate basic validation from proto field options (min/max, regex, etc.)

## References

- [RFD 153 - Resource Implementation Guidelines](./0153-resource-guidelines.md)
