# resource-gen TODOs 10-17: Design Document

Date: 2026-02-23

## Overview

This document covers the design for resource-gen TODO items 10-17 from the README. The items fall into three groups: registration simplifications (10-13), feature implementations (14-15), and removals (16-17).

## Section 1: Registration Simplifications

### TODO #10: applyGeneratedHandlers — error on conflict

Currently `applyGeneratedHandlers` in `tool/tctl/common/resources/resource_gen_registry.go` silently overwrites manual handlers when a generated handler registers a kind that already exists in the base map.

**Change**: Panic if a generated handler's kind conflicts with an existing manual handler. This catches accidental collisions at startup.

Files changed:
- `tool/tctl/common/resources/resource_gen_registry.go` — add conflict check in `applyGeneratedHandlers`

### TODO #11: Move kind into Handler struct

Currently `RegisterGeneratedHandler("foo", func() Handler{...})` takes the kind as a separate argument. The kind is redundant since the Handler could carry it.

**Change**: Add a `Kind string` field to the `Handler` struct. Change `RegisterGeneratedHandler` to take just `func() Handler` and read the kind from the returned value. Update the tctl registration template to include `kind: resourceKind` in the Handler literal.

Files changed:
- `tool/tctl/common/resources/resource.go` — add `Kind` field to `Handler`
- `tool/tctl/common/resources/resource_gen_registry.go` — change `RegisterGeneratedHandler` signature
- `build.assets/tooling/cmd/resource-gen/generators/templates/tctl_registration.go.tmpl` — add `kind` field, remove string arg

### TODO #12: Generate types.Kind* constants

Currently generated code uses bare string literals like `"foo"` for the resource kind. The codebase convention is `types.KindFoo` constants in `api/types/constants.go`.

**Change**: Add a new cross-resource generator that produces `api/types/constants.gen.go` with all generated Kind constants. All generated templates reference `types.KindFoo` instead of bare strings.

This requires a new generator type in the pipeline — one that runs once across all specs rather than per-spec. The `main.go` pipeline adds a step after per-resource generation to collect all specs and emit the single constants file.

Files changed:
- `build.assets/tooling/cmd/resource-gen/main.go` — add cross-resource generation step
- `build.assets/tooling/cmd/resource-gen/generators/registry.go` — add new generator entry or separate function
- New template: `build.assets/tooling/cmd/resource-gen/generators/templates/kind_constants.go.tmpl`
- `build.assets/tooling/cmd/resource-gen/generators/common.go` — add `KindConst` field to `resourceBase`
- All templates that use bare kind strings — reference `types.KindFoo` or the local `resourceKind` const instead
- Specifically: `cache_registration.go.tmpl`, `local_parser.go.tmpl`, `tctl_registration.go.tmpl`

### TODO #13: Remove redundant compile-time check

The `auth_registration.go.tmpl` template has a compile-time check `var _ = func(s Services) services.Foos { return s }` that is redundant — the `Backend: cfg.AuthServer.Services` line in the init function already won't compile if Services doesn't implement `services.Foos`.

**Change**: Remove the `var _` line and its comment from the template. Keep the `Reader: cfg.AuthServer.Cache` comment about authclient.Cache embedding (that one is less obvious from the code alone).

Files changed:
- `build.assets/tooling/cmd/resource-gen/generators/templates/auth_registration.go.tmpl`

## Section 2: Feature Implementations

### TODO #14: emit_on_get — generate Get audit events

The spec already has `EmitOnGet` field, defaults don't enable it, and validation rejects it with `NotImplemented`. The gRPC service template's Get handler has no audit emission.

**Changes**:

1. Remove the `NotImplemented` guard for `EmitOnGet` in `spec.go`
2. Update `HasAudit` computation in `common.go` to include `Get && EmitOnGet`
3. Add `emitGetAuditEvent` method to `grpc_service_custom.go.tmpl` (scaffold), following the same pattern as create/update/delete audit methods
4. Add audit emission call to Get handler in `grpc_service.go.tmpl`. The Get handler currently discards the authz context (`_, err := s.authorize`); when `EmitOnGet` is true, it captures it (`authCtx, err := s.authorize`) for the audit event
5. The audit event uses the same structure: `apievents.FooGet` with Metadata, UserMetadata, ConnectionMetadata, Status, ResourceMetadata

Files changed:
- `build.assets/tooling/cmd/resource-gen/spec/spec.go` — remove NotImplemented check
- `build.assets/tooling/cmd/resource-gen/generators/common.go` — update HasAudit
- `build.assets/tooling/cmd/resource-gen/generators/templates/grpc_service.go.tmpl` — add audit call to Get
- `build.assets/tooling/cmd/resource-gen/generators/templates/grpc_service_custom.go.tmpl` — add emitGetAuditEvent stub

### TODO #15: Integration test for lifecycle hooks

The hook infrastructure is generated (BeforeCreate/AfterCreate etc.) but there's no test proto that exercises it.

**Change**: Add a test proto with `hooks: {enable_lifecycle_hooks: true}` and write generator output tests that verify:
- Generated code contains `Hooks` struct with expected fields (BeforeCreate, AfterCreate, etc.)
- Generated gRPC service includes hook call sites (`s.hooks != nil && s.hooks.BeforeCreate != nil`)
- ServiceConfig includes `Hooks *Hooks` field
- Hook fields match the operations defined in the proto

This is a generator output test (content assertion on generated code), similar to existing tests in `generator_test.go`.

Files changed:
- `build.assets/tooling/cmd/resource-gen/generators/generator_test.go` — add hook-related test cases
- Possibly a new test fixture proto file

## Section 3: Removals

### TODO #16: Remove immutable flag

The `immutable` flag in `BehaviorConfig` is redundant — if a resource shouldn't support updates, the developer simply omits Update/Upsert RPCs from the proto service definition.

**Changes**:

1. Remove `immutable` field from `OperationsConfig` in `api/proto/teleport/options/v1/resource.proto`
2. Remove `BehaviorConfig` struct and `Behavior` field from `ResourceSpec` in `spec/spec.go`
3. Remove `cfg.Operations.GetImmutable()` handling in `parser/defaults.go`
4. Remove `{{- if .Immutable}}` conditional branches from `backend_impl.go.tmpl` — only normal Update/Upsert paths remain
5. Remove `Immutable` field from `backendImplData` in `generators/backend_impl.go`
6. Remove immutable-related test cases in `generators/generator_test.go`
7. Remove immutable-related validation in `spec.go`

Files changed:
- `api/proto/teleport/options/v1/resource.proto`
- `build.assets/tooling/cmd/resource-gen/spec/spec.go`
- `build.assets/tooling/cmd/resource-gen/parser/defaults.go`
- `build.assets/tooling/cmd/resource-gen/generators/templates/backend_impl.go.tmpl`
- `build.assets/tooling/cmd/resource-gen/generators/backend_impl.go`
- `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`

### TODO #17: Remove load_secrets from scope

The `load_secrets` feature is too resource-specific to generalize. Only 5 hand-written resources use it, and they each have custom patterns. Generated resources typically don't have secret fields.

**Changes**:

1. Remove `LoadSecrets` field from `CacheConfig` in `spec/spec.go`
2. Remove the `rs.Cache.LoadSecrets = cfg.Cache.GetLoadSecrets()` line in `parser/defaults.go`
3. Remove the `NotImplemented("cache.load_secrets")` check in `spec.go`
4. Remove the `load_secrets` field from `CacheConfig` in `resource.proto` (or leave for future use)
5. Remove the `LoadSecrets: false` default in `parser/defaults.go`

The generated cache fetcher continues to ignore the `loadSecrets` parameter, which is correct for most resources. Resources that need secrets handling customize the cache registration manually.

Files changed:
- `build.assets/tooling/cmd/resource-gen/spec/spec.go`
- `build.assets/tooling/cmd/resource-gen/parser/defaults.go`
- `api/proto/teleport/options/v1/resource.proto`

## Implementation Order

Recommended order to minimize conflicts:

1. **Removals first** (16, 17) — simplifies the codebase before adding features
2. **Registration simplifications** (13, 10, 11) — straightforward changes
3. **Kind constants generation** (12) — cross-cutting, affects many templates
4. **emit_on_get** (14) — new feature, builds on simplified codebase
5. **Lifecycle hooks test** (15) — verification, can be done last
