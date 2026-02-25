# resource-gen TODOs 10-17 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement all remaining resource-gen TODO items 10-17: registration simplifications, emit_on_get audit events, lifecycle hooks test, and removal of immutable/load_secrets.

**Architecture:** resource-gen is a proto-driven code generator in `build.assets/tooling/cmd/resource-gen/`. Changes span three layers: the spec/validation model (`spec/`), the template-based generators (`generators/`), and the runtime registration infrastructure (`lib/auth/`, `lib/cache/`, `tool/tctl/`). A new cross-resource generator produces a shared `api/types/constants.gen.go` file.

**Tech Stack:** Go, text/template, protobuf, testify

**Test command:** `cd build.assets/tooling && go test ./cmd/resource-gen/...`

**Design doc:** `docs/plans/2026-02-23-resource-gen-todos-10-17-design.md`

---

### Task 1: Remove immutable flag (TODO #16)

**Files:**
- Modify: `api/proto/teleport/options/v1/resource.proto:106-110`
- Modify: `build.assets/tooling/cmd/resource-gen/spec/spec.go:86-93`
- Modify: `build.assets/tooling/cmd/resource-gen/parser/defaults.go:125-127`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/backend_impl.go:26-32`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/templates/backend_impl.go.tmpl:79-105`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/generator_test.go:154-162`

**Step 1: Remove immutable test case**

In `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`, delete the `TestGenerateBackendImplementationImmutable` function (lines 154-162):

```go
// DELETE this entire function:
func TestGenerateBackendImplementationImmutable(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Upsert: true})
	rs.Behavior = spec.BehaviorConfig{Immutable: true}

	got, err := GenerateBackendImplementation(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "foos are immutable and cannot be updated")
	require.Contains(t, got, "foos are immutable and cannot be upserted")
}
```

**Step 2: Run tests — verify immutable test is gone**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/generators/ -run TestGenerateBackendImplementationImmutable -v`
Expected: "no test to run"

**Step 3: Remove BehaviorConfig from spec**

In `build.assets/tooling/cmd/resource-gen/spec/spec.go`:
- Delete the `BehaviorConfig` struct (lines 90-93)
- Delete the `Behavior BehaviorConfig` field from `ResourceSpec` (line 49)

**Step 4: Remove immutable from parser defaults**

In `build.assets/tooling/cmd/resource-gen/parser/defaults.go`:
- Delete the `Behavior` default (lines 61-63):
  ```go
  // DELETE:
  Behavior: spec.BehaviorConfig{
      Immutable: false,
  },
  ```
- Delete the `cfg.Operations.GetImmutable()` line (lines 125-127):
  ```go
  // DELETE:
  if cfg.Operations != nil {
      rs.Behavior.Immutable = cfg.Operations.GetImmutable()
  }
  ```

**Step 5: Remove Immutable from backend generator**

In `build.assets/tooling/cmd/resource-gen/generators/backend_impl.go`:
- Delete `Immutable bool` from `backendImplData` (line 29)
- Delete `Immutable: rs.Behavior.Immutable` from the data initialization (line 43)

**Step 6: Remove immutable branches from backend template**

In `build.assets/tooling/cmd/resource-gen/generators/templates/backend_impl.go.tmpl`:
- Lines 79-91: Replace the `{{- if .Ops.Update}}` block to remove the immutable conditional. Keep only the non-immutable path:

Before:
```
{{- if .Ops.Update}}
{{- if .Immutable}}
...immutable rejection...
{{- else}}
...normal update...
{{- end}}
{{- end}}
```

After:
```
{{- if .Ops.Update}}

func (s *{{.Kind}}Service) Update{{.Kind}}(ctx context.Context, {{.Lower}} {{.QualType}}) ({{.QualType}}, error) {
	r, err := s.service.ConditionalUpdateResource(ctx, {{.Lower}})
	return r, trace.Wrap(err)
}
{{- end}}
```

Same for Upsert (lines 93-105):

After:
```
{{- if .Ops.Upsert}}

func (s *{{.Kind}}Service) Upsert{{.Kind}}(ctx context.Context, {{.Lower}} {{.QualType}}) ({{.QualType}}, error) {
	r, err := s.service.UpsertResource(ctx, {{.Lower}})
	return r, trace.Wrap(err)
}
{{- end}}
```

**Step 7: Remove immutable from proto**

In `api/proto/teleport/options/v1/resource.proto`:
- Delete the `immutable` field from `OperationsConfig` (line 109). If OperationsConfig has no other fields, delete the entire message and remove the `operations` field from `ResourceConfig`.

Since `OperationsConfig` only has `immutable`, delete the entire message (lines 106-110) and the `operations` field from `ResourceConfig` (line 30).

**Step 8: Regenerate proto Go code**

Run: `cd api/proto && buf generate teleport/options/v1/resource.proto`
(Or use whatever proto generation command the project uses — check Makefile.)

NOTE: If buf is not available or proto regen is complex, leave a TODO comment and skip this step. The Go code may still compile if the generated proto Go code already exists without the `immutable` field being referenced anywhere.

**Step 9: Run all tests**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/...`
Expected: All pass

**Step 10: Commit**

```bash
git add -A build.assets/tooling/cmd/resource-gen/ api/proto/teleport/options/v1/resource.proto
git commit -m "resource-gen: remove immutable flag (TODO #16)

The immutable behavior is redundant — resources that shouldn't support
updates simply omit Update/Upsert RPCs from the proto definition."
```

---

### Task 2: Remove load_secrets from scope (TODO #17)

**Files:**
- Modify: `build.assets/tooling/cmd/resource-gen/spec/spec.go:63-67`
- Modify: `build.assets/tooling/cmd/resource-gen/spec/spec_test.go:102-112`
- Modify: `build.assets/tooling/cmd/resource-gen/parser/defaults.go:44-45,92`
- Modify: `api/proto/teleport/options/v1/resource.proto:73`

**Step 1: Remove load_secrets test case**

In `build.assets/tooling/cmd/resource-gen/spec/spec_test.go`, delete the "load_secrets not implemented" test case (lines 102-112):

```go
// DELETE this test case from the table:
{
    name: "load_secrets not implemented",
    spec: ResourceSpec{
        ServiceName: "teleport.foo.v1.FooService",
        Kind:        "foo",
        Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternStandard},
        Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
        Cache:       CacheConfig{LoadSecrets: true},
    },
    wantErr: require.Error,
},
```

**Step 2: Remove LoadSecrets from CacheConfig**

In `build.assets/tooling/cmd/resource-gen/spec/spec.go`:
- Delete `LoadSecrets bool` from `CacheConfig` (line 66)
- Delete the `NotImplemented("cache.load_secrets")` validation check (lines 147-149):
  ```go
  // DELETE:
  if s.Cache.LoadSecrets {
      return trace.NotImplemented("cache.load_secrets is not yet supported by resource-gen")
  }
  ```

**Step 3: Remove LoadSecrets from parser defaults**

In `build.assets/tooling/cmd/resource-gen/parser/defaults.go`:
- Delete `LoadSecrets: false` from the default CacheConfig (line 45)
- Delete `rs.Cache.LoadSecrets = cfg.Cache.GetLoadSecrets()` (line 92)

**Step 4: Remove load_secrets from proto**

In `api/proto/teleport/options/v1/resource.proto`:
- Delete the `load_secrets` field from `CacheConfig` (line 73):
  ```protobuf
  // DELETE:
  bool load_secrets = 3;
  ```

**Step 5: Run tests**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/...`
Expected: All pass

**Step 6: Commit**

```bash
git add -A build.assets/tooling/cmd/resource-gen/ api/proto/teleport/options/v1/resource.proto
git commit -m "resource-gen: remove load_secrets from scope (TODO #17)

Only 5 hand-written resources use loadSecrets and each has a custom
pattern. Generated resources don't have secret fields; the rare one
that does can customize the cache registration manually."
```

---

### Task 3: Remove redundant compile-time check (TODO #13)

**Files:**
- Modify: `build.assets/tooling/cmd/resource-gen/generators/templates/auth_registration.go.tmpl:12-14`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/generator_test.go:280-282`

**Step 1: Update auth registration test**

In `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`, in `TestGenerateAuthRegistration`:
- Remove the assertions about the compile-time check (lines 280-282):
  ```go
  // DELETE:
  require.Contains(t, got, "GENERATED CHECK")
  require.Contains(t, got, "add services.Foos to the Services interface in lib/auth/services.go")
  require.Contains(t, got, "var _ = func(s Services) services.Foos { return s }")
  ```
- Add assertion that the redundant check is NOT present:
  ```go
  require.NotContains(t, got, "var _ = func(s Services)")
  ```

**Step 2: Run test — verify it fails (TDD)**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/generators/ -run TestGenerateAuthRegistration -v`
Expected: FAIL (template still has the old check)

**Step 3: Remove redundant check from template**

In `build.assets/tooling/cmd/resource-gen/generators/templates/auth_registration.go.tmpl`, delete lines 12-14:

```go
// DELETE these lines:
// GENERATED CHECK: This line ensures Services embeds services.{{.Plural}}.
// If this fails to compile, add services.{{.Plural}} to the Services interface in lib/auth/services.go.
var _ = func(s Services) services.{{.Plural}} { return s }
```

Also remove the `"{{.Module}}/lib/services"` import since it's only used by the deleted check. Keep the other imports.

**Step 4: Run test — verify it passes**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/generators/ -run TestGenerateAuthRegistration -v`
Expected: PASS

**Step 5: Run all tests**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/...`
Expected: All pass

**Step 6: Commit**

```bash
git add -A build.assets/tooling/cmd/resource-gen/
git commit -m "resource-gen: remove redundant compile-time check from auth registration (TODO #13)

The Backend: cfg.AuthServer.Services line already enforces that
Services implements the resource interface at compile time."
```

---

### Task 4: Error on handler conflict (TODO #10)

**Files:**
- Modify: `tool/tctl/common/resources/resource_gen_registry.go:54-65`

**Step 1: Add conflict detection to applyGeneratedHandlers**

In `tool/tctl/common/resources/resource_gen_registry.go`, modify `applyGeneratedHandlers` (line 54-65):

Before:
```go
func applyGeneratedHandlers(base map[string]Handler) map[string]Handler {
	generatedHandlers.mu.RLock()
	defer generatedHandlers.mu.RUnlock()

	if len(generatedHandlers.m) == 0 {
		return base
	}
	for kind, factory := range generatedHandlers.m {
		base[kind] = factory() // TODO: avoid overriding?
	}
	return base
}
```

After:
```go
func applyGeneratedHandlers(base map[string]Handler) map[string]Handler {
	generatedHandlers.mu.RLock()
	defer generatedHandlers.mu.RUnlock()

	if len(generatedHandlers.m) == 0 {
		return base
	}
	for kind, factory := range generatedHandlers.m {
		if _, exists := base[kind]; exists {
			panic("resources: generated handler conflicts with existing handler for kind " + kind)
		}
		base[kind] = factory()
	}
	return base
}
```

**Step 2: Run existing tests to verify no regressions**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/...`
Expected: All pass (this change is in tool/tctl, not in the generator tests — generator tests don't exercise the runtime registry)

**Step 3: Commit**

```bash
git add tool/tctl/common/resources/resource_gen_registry.go
git commit -m "tctl: panic on generated handler conflict with manual handler (TODO #10)

Previously generated handlers silently overwrote manual handlers for
the same kind. Now panics at startup if a collision is detected."
```

---

### Task 5: Move kind into Handler struct (TODO #11)

**Files:**
- Modify: `tool/tctl/common/resources/resource.go:106-114`
- Modify: `tool/tctl/common/resources/resource_gen_registry.go:24-51`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/templates/tctl_registration.go.tmpl:97-121`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/generator_test.go:368`

**Step 1: Add Kind field to Handler struct**

In `tool/tctl/common/resources/resource.go`, add `kind` to the Handler struct (after line 113):

```go
type Handler struct {
	kind          string  // <-- ADD THIS
	getHandler    func(context.Context, *authclient.Client, services.Ref, GetOpts) (Collection, error)
	createHandler func(context.Context, *authclient.Client, services.UnknownResource, CreateOpts) error
	updateHandler func(context.Context, *authclient.Client, services.UnknownResource, CreateOpts) error
	deleteHandler func(context.Context, *authclient.Client, services.Ref) error
	singleton     bool
	mfaRequired   bool
	description   string
}
```

**Step 2: Change RegisterGeneratedHandler signature**

In `tool/tctl/common/resources/resource_gen_registry.go`:

Before:
```go
type GeneratedHandlerFactory func() Handler

func RegisterGeneratedHandler(kind string, factory GeneratedHandlerFactory) {
	if kind == "" {
		panic("resources: handler kind is required")
	}
	if factory == nil {
		panic("resources: handler factory is nil")
	}

	generatedHandlers.mu.Lock()
	defer generatedHandlers.mu.Unlock()

	if generatedHandlers.m == nil {
		generatedHandlers.m = make(map[string]GeneratedHandlerFactory)
	}
	if _, exists := generatedHandlers.m[kind]; exists {
		panic("resources: duplicate generated handler for kind " + kind)
	}
	generatedHandlers.m[kind] = factory
}
```

After:
```go
type GeneratedHandlerFactory func() Handler

func RegisterGeneratedHandler(factory GeneratedHandlerFactory) {
	if factory == nil {
		panic("resources: handler factory is nil")
	}

	// Call factory once to extract the kind for registration.
	h := factory()
	if h.kind == "" {
		panic("resources: handler factory returned handler with empty kind")
	}

	generatedHandlers.mu.Lock()
	defer generatedHandlers.mu.Unlock()

	if generatedHandlers.m == nil {
		generatedHandlers.m = make(map[string]GeneratedHandlerFactory)
	}
	if _, exists := generatedHandlers.m[h.kind]; exists {
		panic("resources: duplicate generated handler for kind " + h.kind)
	}
	generatedHandlers.m[h.kind] = factory
}
```

**Step 3: Update tctl registration template**

In `build.assets/tooling/cmd/resource-gen/generators/templates/tctl_registration.go.tmpl`, change the `init()` function:

Before (lines 97-121):
```
func init() {
	RegisterGeneratedHandler("{{.Lower}}", func() Handler {
		return Handler{
{{- if .Ops.Get}}
			getHandler:    get{{.Kind}},
{{- end}}
...
		}
	})
}
```

After:
```
func init() {
	RegisterGeneratedHandler(func() Handler {
		return Handler{
			kind:          resourceKind,
{{- if .Ops.Get}}
			getHandler:    get{{.Kind}},
{{- end}}
...
		}
	})
}
```

Note: `resourceKind` is not defined in the tctl template's package. We need to add a local const. Add at the top of the template (after the `func ...Timestamp...` block but before `type {{.Lower}}Collection`):

```
const resourceKind = "{{.Lower}}"
```

**Step 4: Update tctl registration test**

In `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`, the `TestGenerateTCTLRegistration` test:
- Change `require.Contains(t, got, "RegisterGeneratedHandler")` — this still holds
- The test currently checks for `RegisterGeneratedHandler` as a substring which still works since the function name hasn't changed

Also verify the template now includes `kind:` and `resourceKind`:
```go
require.Contains(t, got, `kind:          resourceKind`)
require.Contains(t, got, `const resourceKind = "foo"`)
```

**Step 5: Run tests**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/...`
Expected: All pass

**Step 6: Commit**

```bash
git add tool/tctl/common/resources/resource.go tool/tctl/common/resources/resource_gen_registry.go build.assets/tooling/cmd/resource-gen/
git commit -m "tctl: move kind into Handler struct, simplify registration (TODO #11)

RegisterGeneratedHandler now takes just a factory function. The kind
is read from the Handler struct returned by the factory, eliminating
the redundant string argument."
```

---

### Task 6: Generate types.Kind* constants (TODO #12)

**Files:**
- Create: `build.assets/tooling/cmd/resource-gen/generators/templates/kind_constants.go.tmpl`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/templates.go`
- Create: `build.assets/tooling/cmd/resource-gen/generators/kind_constants.go`
- Modify: `build.assets/tooling/cmd/resource-gen/main.go:148-172`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/common.go:43-44`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/templates/cache_registration.go.tmpl:27`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/templates/local_parser.go.tmpl:14`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`

**Step 1: Write test for kind constants generator**

In `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`, add:

```go
func TestGenerateKindConstants(t *testing.T) {
	specs := []spec.ResourceSpec{
		testSpec(spec.OperationSet{Get: true, List: true}),
	}
	// Override kind for variety
	specs[0].Kind = "widget"
	specs[0].ServiceName = "teleport.widget.v1.WidgetService"

	got, err := GenerateKindConstants(specs)
	require.NoError(t, err)
	require.Contains(t, got, "package types")
	require.Contains(t, got, `KindWidget = "widget"`)
}

func TestGenerateKindConstantsMultiple(t *testing.T) {
	specs := []spec.ResourceSpec{
		testSpec(spec.OperationSet{Get: true, List: true}),
	}
	spec2 := testSpec(spec.OperationSet{Get: true})
	spec2.Kind = "gadget"
	spec2.ServiceName = "teleport.gadget.v1.GadgetService"
	specs = append(specs, spec2)

	got, err := GenerateKindConstants(specs)
	require.NoError(t, err)
	require.Contains(t, got, `KindFoo = "foo"`)
	require.Contains(t, got, `KindGadget = "gadget"`)
}
```

**Step 2: Run test — verify it fails**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/generators/ -run TestGenerateKindConstants -v`
Expected: FAIL — `GenerateKindConstants` doesn't exist yet

**Step 3: Create kind constants template**

Create `build.assets/tooling/cmd/resource-gen/generators/templates/kind_constants.go.tmpl`:

```
package types

// Generated kind constants for resource-gen managed resources.
// DO NOT EDIT: This file is generated by resource-gen.
const (
{{- range .Kinds}}
	Kind{{.ExportedName}} = "{{.Lower}}"
{{- end}}
)
```

**Step 4: Create kind constants generator**

Create `build.assets/tooling/cmd/resource-gen/generators/kind_constants.go`:

```go
package generators

import (
	"sort"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

type kindEntry struct {
	ExportedName string
	Lower        string
}

type kindConstantsData struct {
	Kinds []kindEntry
}

var kindConstantsTmpl = mustReadTemplate("kind_constants.go.tmpl")

// GenerateKindConstants renders a single file with Kind* constants for all
// generated resources.
func GenerateKindConstants(specs []spec.ResourceSpec) (string, error) {
	entries := make([]kindEntry, 0, len(specs))
	for _, rs := range specs {
		entries = append(entries, kindEntry{
			ExportedName: exportedName(rs.Kind),
			Lower:        rs.Kind,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Lower < entries[j].Lower
	})

	data := kindConstantsData{Kinds: entries}
	out, err := render("kindConstants", kindConstantsTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}
```

**Step 5: Run test — verify it passes**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/generators/ -run TestGenerateKindConstants -v`
Expected: PASS

**Step 6: Wire into main.go pipeline**

In `build.assets/tooling/cmd/resource-gen/main.go`, modify `generateFiles` to also produce the constants file:

After the per-resource loop (after line 170), add:

```go
// Cross-resource: kind constants file
if len(specs) > 0 {
    content, err := generators.GenerateKindConstants(specs)
    if err != nil {
        return nil, trace.Wrap(err, "generating kind constants")
    }
    files = append(files, generatedFile{
        Path:    filepath.Join("api", "types", "constants.gen.go"),
        Content: content,
    })
}
```

**Step 7: Update templates to use types.Kind* constants**

In `cache_registration.go.tmpl` (line 27), change:
```
types.WatchKind{Kind: "{{.Lower}}"}
```
to:
```
types.WatchKind{Kind: types.Kind{{.Kind}}}
```

Wait — `types.Kind{{.Kind}}` would expand to `types.KindFoo`. But the cache template already imports `types`. This should work.

In `local_parser.go.tmpl` (line 14), change:
```
RegisterGeneratedResourceParser("{{.Lower}}",
```
to:
```
RegisterGeneratedResourceParser(types.Kind{{.Kind}},
```

The local_parser template already imports `types`, so this works.

In `tctl_registration.go.tmpl`, the `resourceKind` const added in Task 5 should reference the types constant:
```
// Before:
const resourceKind = "{{.Lower}}"
// After:
const resourceKind = types.Kind{{.Kind}}
```

But wait — the tctl template generates code in `package resources` which already imports `types`. This works.

In `grpc_service.go.tmpl` (line 27):
```
// Before:
const resourceKind = "{{.Lower}}"
// After: keep as string literal — gRPC service package should not import types
```

Actually, the gRPC service is in its own package (e.g. `package foov1`) and already imports types. So we can change this too. But it's a deeper dependency. Leave the gRPC service const as a string literal for now — it's only used within the package and doesn't need to reference the types constant.

**Step 8: Update affected tests**

Update `TestGenerateCacheRegistration` to check for `types.KindFoo` instead of bare string:
```go
// Before:
// (no specific assertion about "foo" in WatchKind — just checks structure)
// After: add
require.Contains(t, got, "types.KindFoo")
require.NotContains(t, got, `Kind: "foo"`)
```

Update `TestGenerateLocalParserRegistration`:
```go
require.Contains(t, got, "types.KindFoo")
```

Update `TestGenerateTCTLRegistration`:
```go
require.Contains(t, got, "types.KindFoo")
```

**Step 9: Run all tests**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/...`
Expected: All pass

**Step 10: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/
git commit -m "resource-gen: generate types.Kind* constants (TODO #12)

New cross-resource generator produces api/types/constants.gen.go with
Kind constants for all generated resources. Templates updated to
reference types.KindFoo instead of bare string literals."
```

---

### Task 7: Implement emit_on_get audit events (TODO #14)

**Files:**
- Modify: `build.assets/tooling/cmd/resource-gen/spec/spec.go:144-146`
- Modify: `build.assets/tooling/cmd/resource-gen/spec/spec_test.go:91-101`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/common.go:64-68`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/grpc_service.go:28-33`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/templates/grpc_service.go.tmpl:64-95`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/templates/grpc_service_custom.go.tmpl:101-122`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`

**Step 1: Write failing test for emit_on_get in spec validation**

In `build.assets/tooling/cmd/resource-gen/spec/spec_test.go`, change the "emit_on_get not implemented" test case to expect success:

```go
// CHANGE wantErr from require.Error to require.NoError:
{
    name: "emit_on_get is valid",
    spec: ResourceSpec{
        ServiceName: "teleport.foo.v1.FooService",
        Kind:        "foo",
        Storage:     StorageConfig{BackendPrefix: "foos", Pattern: StoragePatternStandard},
        Pagination:  PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
        Audit:       AuditConfig{EmitOnGet: true},
        Operations:  OperationSet{Get: true},
    },
    wantErr: require.NoError,
},
```

**Step 2: Run test — verify it fails**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/spec/ -run TestSpecValidate/emit_on_get -v`
Expected: FAIL (still returns NotImplemented)

**Step 3: Remove NotImplemented guard**

In `build.assets/tooling/cmd/resource-gen/spec/spec.go`, delete lines 144-146:
```go
// DELETE:
if s.Audit.EmitOnGet {
    return trace.NotImplemented("audit.emit_on_get is not yet supported by resource-gen")
}
```

**Step 4: Run spec test — verify it passes**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/spec/ -run TestSpecValidate/emit_on_get -v`
Expected: PASS

**Step 5: Write failing test for generated Get audit event**

In `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`, add:

```go
func TestGenerateGRPCServiceWithEmitOnGet(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true})
	rs.Audit.EmitOnGet = true

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "s.emitGetAuditEvent(ctx,")
	// Get handler must capture authCtx when emitting
	require.Contains(t, got, "authCtx, err := s.authorize(ctx, types.VerbRead)")
}

func TestGenerateGRPCServiceWithoutEmitOnGet(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true})
	rs.Audit.EmitOnGet = false

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)
	require.NotContains(t, got, "emitGetAuditEvent")
	// Without emit, Get handler discards authCtx
	require.Contains(t, got, "_, err := s.authorize(ctx, types.VerbRead)")
}

func TestGenerateGRPCServiceCustomWithEmitOnGet(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true})
	rs.Audit.EmitOnGet = true

	got, err := GenerateGRPCServiceCustom(rs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "func (s *Service) emitGetAuditEvent(")
	require.Contains(t, got, "&apievents.FooGet{")
	require.Contains(t, got, "libevents.FooGetEvent")
	require.Contains(t, got, "libevents.FooGetCode")
}
```

**Step 6: Run test — verify it fails**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/generators/ -run TestGenerateGRPCServiceWithEmitOnGet -v`
Expected: FAIL

**Step 7: Update HasAudit computation**

In `build.assets/tooling/cmd/resource-gen/generators/common.go`, update `newResourceBase` (lines 64-68):

Before:
```go
HasAudit: (rs.Operations.Create && rs.Audit.EmitOnCreate) ||
    (rs.Operations.Update && rs.Audit.EmitOnUpdate) ||
    (rs.Operations.Upsert && rs.Audit.EmitOnUpdate) ||
    (rs.Operations.Delete && rs.Audit.EmitOnDelete),
```

After:
```go
HasAudit: (rs.Operations.Get && rs.Audit.EmitOnGet) ||
    (rs.Operations.Create && rs.Audit.EmitOnCreate) ||
    (rs.Operations.Update && rs.Audit.EmitOnUpdate) ||
    (rs.Operations.Upsert && rs.Audit.EmitOnUpdate) ||
    (rs.Operations.Delete && rs.Audit.EmitOnDelete),
```

**Step 8: Update grpc_service.go.tmpl — Get handler**

In `build.assets/tooling/cmd/resource-gen/generators/templates/grpc_service.go.tmpl`, modify the Get handler.

For the standard (non-singleton) Get handler (currently lines 82-94):

Before:
```
func (s *Service) Get{{.Kind}}(ctx context.Context, req *{{.ProtoAlias}}.Get{{.Kind}}Request) ({{.QualType}}, error) {
	_, err := s.authorize(ctx, types.VerbRead)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rsp, err := s.reader.Get{{.Kind}}(ctx, req.GetName())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}
```

After:
```
func (s *Service) Get{{.Kind}}(ctx context.Context, req *{{.ProtoAlias}}.Get{{.Kind}}Request) ({{.QualType}}, error) {
{{- if .Audit.EmitOnGet}}
	authCtx, err := s.authorize(ctx, types.VerbRead)
{{- else}}
	_, err := s.authorize(ctx, types.VerbRead)
{{- end}}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rsp, err := s.reader.Get{{.Kind}}(ctx, req.GetName())
{{- if .Audit.EmitOnGet}}
	s.emitGetAuditEvent(ctx, req.GetName(), authCtx, err)
{{- end}}
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return rsp, nil
}
```

Do the same for the singleton Get handler (lines 69-79), using an empty string or the singleton name for the resource name.

**Step 9: Update grpc_service_custom.go.tmpl — add emitGetAuditEvent**

In `build.assets/tooling/cmd/resource-gen/generators/templates/grpc_service_custom.go.tmpl`, add after the existing audit methods (at the end of the file, before the closing `{{- end}}`):

```
{{- if and .Ops.Get .Audit.EmitOnGet}}

func (s *Service) emitGetAuditEvent(ctx context.Context, name string, authCtx *authz.Context, err error) {
	if auditErr := s.emitter.EmitAuditEvent(ctx, &apievents.{{.Kind}}Get{
		Metadata: apievents.Metadata{
			Type: libevents.{{.Kind}}GetEvent,
			Code: libevents.{{.Kind}}GetCode,
		},
		UserMetadata:       authCtx.GetUserMetadata(),
		ConnectionMetadata: authz.ConnectionMetadata(ctx),
		Status:             eventStatus(err),
		ResourceMetadata: apievents.ResourceMetadata{
			Name: name,
		},
	}); auditErr != nil {
		slog.WarnContext(ctx, "Failed to emit {{.Lower}} get event.", "error", auditErr)
	}
}
{{- end}}
```

**Step 10: Run tests — verify they pass**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/... -v`
Expected: All pass

**Step 11: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/
git commit -m "resource-gen: implement emit_on_get audit events (TODO #14)

When audit.emit_on_get is true, the generated Get handler now emits
an audit event with the same structure as create/update/delete events.
The Get handler captures the auth context for the audit event metadata."
```

---

### Task 8: Add lifecycle hooks integration test (TODO #15)

**Files:**
- Modify: `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`

**Step 1: Write comprehensive hooks tests**

The existing `TestGenerateGRPCServiceWithHooks` and `TestGenerateGRPCServiceCustomWithHooks` tests already cover the gRPC service template. Add tests for the scaffold (custom) template to verify hook wiring end-to-end:

```go
func TestGenerateGRPCServiceCustomWithHooksAllOps(t *testing.T) {
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Update: true, Upsert: true, Delete: true})
	rs.Hooks = spec.HooksConfig{EnableLifecycleHooks: true}

	got, err := GenerateGRPCServiceCustom(rs, testModule)
	require.NoError(t, err)

	// ServiceConfig has Hooks field
	require.Contains(t, got, "Hooks      *Hooks")
	// Service struct has hooks field
	require.Contains(t, got, "hooks      *Hooks")
	// Constructor wires hooks
	require.Contains(t, got, "hooks:      cfg.Hooks,")
}

func TestGenerateGRPCServiceHooksMatchOps(t *testing.T) {
	// Only Create and Delete — no Update/Upsert hooks
	rs := testSpec(spec.OperationSet{Get: true, List: true, Create: true, Delete: true})
	rs.Hooks = spec.HooksConfig{EnableLifecycleHooks: true}

	got, err := GenerateGRPCService(rs, testModule)
	require.NoError(t, err)

	// Create hooks present
	require.Contains(t, got, "BeforeCreate func(context.Context,")
	require.Contains(t, got, "AfterCreate  func(context.Context,")
	// Delete hooks present
	require.Contains(t, got, "BeforeDelete func(context.Context, string) error")
	require.Contains(t, got, "AfterDelete  func(context.Context, string)")
	// Update/Upsert hooks absent (ops not enabled)
	require.NotContains(t, got, "BeforeUpdate")
	require.NotContains(t, got, "AfterUpdate")
	require.NotContains(t, got, "BeforeUpsert")
	require.NotContains(t, got, "AfterUpsert")
}
```

**Step 2: Run tests — should pass since hooks are already implemented**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/generators/ -run TestGenerateGRPCService.*Hook -v`
Expected: All PASS (these are validating existing behavior)

**Step 3: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/generators/generator_test.go
git commit -m "resource-gen: add lifecycle hooks integration tests (TODO #15)

Adds tests verifying hook generation matches operation set: only
operations enabled in the proto get Before/After hook fields.
Also verifies ServiceConfig wiring for all-ops case."
```

---

### Task 9: Update README and final cleanup

**Files:**
- Modify: `build.assets/tooling/cmd/resource-gen/README.md`

**Step 1: Update README**

Replace the TODO list in `build.assets/tooling/cmd/resource-gen/README.md` with completed status:

```markdown
# resource-gen

Proto-driven code generator for Teleport resources.

## Completed TODOs

- [x] 10. `applyGeneratedHandlers` — panics on conflict with manual handler
- [x] 11. `RegisterGeneratedHandler` — kind moved into Handler struct
- [x] 12. `RegisterGeneratedCollectionBuilder` — uses types.Kind* constants
- [x] 13. `RegisterGeneratedGRPCService` — redundant compile-time check removed
- [x] 14. `emit_on_get` — generates audit events on Get
- [x] 15. `enable_lifecycle_hooks` — covered by integration tests
- [x] 16. `immutable` — removed (use proto to control operations instead)
- [x] 17. `load_secrets` — removed from scope

## Remaining TODOs

1. Drop `_custom` from filenames
2. Generate events stuff (types + constants)
3. Add proper `types.FooType` constant
4. Simplify registration code patterns
5. Fix tctl MFA handling
```

**Step 2: Run final test suite**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/...`
Expected: All pass

**Step 3: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/README.md
git commit -m "resource-gen: update README with completed TODO items 10-17"
```
