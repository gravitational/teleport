# Friction Log: AccessPolicy (Scoped Resource) End-to-End Test

**Date:** 2026-02-25
**Resource:** AccessPolicy (scoped by `namespace`, cache disabled)
**Config features exercised:** scoped storage, all CRUD + upsert, all audit events (create/update/delete/get), lifecycle hooks, MFA required, custom tctl columns with verbose, custom pagination, cache disabled

## Pipeline Summary

| Phase | Command | Result |
|-------|---------|--------|
| Event injection | `make generate-resource-events/host` | OK |
| Proto lint + format | `make protos/all` | OK |
| Proto stub generation | `build.assets/genproto.sh` | OK |
| Go code generation | `make generate-resource-services/host` | OK |
| Full build (`make all`) | First attempt: **FAIL**. Second attempt after fix: **PASS** |
| Validation tests | `go test ./lib/services/ -run TestValidateAccessPolicy` | FAIL then PASS after implementing validation |
| Resource-gen unit tests | `go test ./cmd/resource-gen/...` | PASS |

## Bugs Found and Fixed

### Bug 1: Auth registration template always wires Reader to Cache (BLOCKING)

**Severity:** Blocking — compilation fails.

**Symptom:**
```
lib/auth/accesspolicy_register.gen.go:19:17: cannot use cfg.AuthServer.Cache
  as Reader value: authclient.Cache does not implement Reader (missing method GetAccessPolicy)
```

**Root cause:** `auth_registration.go.tmpl` unconditionally uses `cfg.AuthServer.Cache` as the `Reader` parameter for the gRPC service. For cache-disabled resources, the Cache type has no accessor methods for this resource, so the type assertion fails.

**Fix:** Added `CacheEnabled` field to `resourceBase` in `common.go`. Updated `auth_registration.go.tmpl` to conditionally use `cfg.AuthServer.Services` (direct backend) when cache is disabled:
```go
{{- if .CacheEnabled}}
    Reader: cfg.AuthServer.Cache,
{{- else}}
    Reader: cfg.AuthServer.Services, // No cache — reads go directly to backend.
{{- end}}
```

**Files modified:**
- `generators/common.go` — added `CacheEnabled` to `resourceBase`
- `generators/templates/auth_registration.go.tmpl` — conditional Reader source
- `generators/generator_test.go` — updated test to set `Cache.Enabled = true`, added `TestGenerateAuthRegistrationNoCache`

**Impact:** Affects ALL resources with `cache: {enabled: false}`. Previously no resources had cache disabled, so this bug was latent. Any new resource with cache disabled would have hit this.

### Issue 2: Scaffold validation always returns NotImplemented (DESIGN)

**Severity:** Non-blocking (by design) but confusing for developer experience.

**Symptom:** Generated `lib/services/accesspolicy.go` contains:
```go
func ValidateAccessPolicy(accesspolicy *accesspolicyv1.AccessPolicy) error {
    return trace.NotImplemented("accesspolicy validation not yet implemented")
}
```

But the generated test (`accesspolicy_test.go`) includes a "valid minimal" case that expects `wantErr: false`. Running `go test ./lib/services/ -run TestValidateAccessPolicy` immediately fails.

**Recommendation:** Either:
1. Generate a minimal working validation (nil checks, name required, spec required) in the scaffold, or
2. Make the "valid minimal" test case expect an error with a TODO comment.

Option 1 is better because the developer gets a green test run right away and can incrementally add resource-specific checks.

## Observations (Not Bugs)

### O1: Grammar in generated comments — "a AccessPolicy"

Throughout generated files, comments say "a accesspolicy" instead of "an accesspolicy". This comes from the templates using `a {{.Lower}}` without article logic. Cosmetic only.

### O2: tctl scoped get uses `ref.SubKind` as namespace

The tctl handler maps `ref.SubKind` to the scope parameter (namespace). Users would type `tctl get accesspolicy/production/my-policy` where `production` is the SubKind (=namespace) and `my-policy` is the Name. This is reasonable but should be documented, since standard resources don't use SubKind this way.

### O3: No list-all-scopes support in tctl

For scoped resources, `tctl get accesspolicy` (no name) returns an error: "resource name is required for scoped resources". There's no way to list all policies across all namespaces from tctl. This is by design (listing requires a scope), but a future enhancement could support `tctl get accesspolicy/production` to list all policies within a specific namespace.

### O4: Delete hooks don't receive scope parameter

The `BeforeDelete` / `AfterDelete` hooks receive `(ctx context.Context, name string)` — only the name, not the namespace. For scoped resources, a hook that needs the namespace can't get it from the callback signature. Minor, since hooks are rarely used and can look up the scope from the resource if needed.

### O5: Local parser OpDelete for scoped resources

The `local_parser.go.tmpl` doesn't differentiate scoped vs. standard resources. For scoped resources with key `access_policies/production/my-policy`, trimming the prefix yields `production/my-policy`, which gets set as `Metadata.Name`. This is harmless for the cache watcher use case (it just needs the raw event), but could be surprising if code inspects the Name field of delete events.

## Files Generated (12 total)

| File | Type | Notes |
|------|------|-------|
| `lib/services/accesspolicy.gen.go` | Always overwritten | Interfaces with scoped signatures |
| `lib/services/local/accesspolicy.gen.go` | Always overwritten | Backend with `WithPrefix(namespace)` |
| `lib/auth/accesspolicy/accesspolicyv1/service.gen.go` | Always overwritten | gRPC handler with scoped Reader |
| `api/client/accesspolicy.gen.go` | Always overwritten | Client with namespace in Get/List/Delete |
| `lib/auth/accesspolicy_register.gen.go` | Always overwritten | Auth wiring (Reader=Services, no cache) |
| `lib/services/local/accesspolicy_register.gen.go` | Always overwritten | Event parser wiring |
| `tool/tctl/common/resources/accesspolicy_register.gen.go` | Always overwritten | tctl with columns, MFA, scoped get/delete |
| `lib/services/accesspolicy.go` | Scaffold (once) | Validation stub |
| `lib/services/accesspolicy_test.go` | Scaffold (once) | Validation test skeleton |
| `lib/auth/accesspolicy/accesspolicyv1/service.go` | Scaffold (once) | ServiceConfig, authorize, audit events |
| No `lib/cache/accesspolicy.gen.go` | N/A | Correctly omitted (cache disabled) |
| No `lib/cache/accesspolicy_register.gen.go` | N/A | Correctly omitted (cache disabled) |

## Cross-Resource Files Updated

- `api/types/constants.gen.go` — `KindAccessPolicy = "accesspolicy"`
- `lib/auth/services.gen.go` — `servicesGenerated` includes `services.AccessPolicies`
- `lib/auth/authclient/api.gen.go` — `cacheGeneratedServices` correctly EXCLUDES AccessPolicy
- `lib/cache/index.gen.go` — `generatedConfig` correctly EXCLUDES AccessPolicy
- `lib/events/api.gen.go` — 4 event type constants
- `lib/events/codes.gen.go` — 4 event code constants (AP001I-AP004I)
- `lib/events/dynamic.gen.go` — 4 dynamic factory registrations
- `lib/events/events_test.gen.go` — 4 test entries
- `api/types/events/oneof.gen.go` — 4 OneOf converter registrations
- `api/proto/teleport/legacy/types/events/events.proto` — 4 event messages + OneOf entries

## Verdict

After fixing Bug 1 (auth registration cache wiring), the full pipeline works end-to-end for a scoped resource with all features enabled. The generated code compiles cleanly across all 5 binaries (teleport, tctl, tsh, tbot, fdpass-teleport). The scoped templates produce correct `WithPrefix(namespace)` backend calls, scoped Reader interface methods, and properly plumbed namespace parameters through the gRPC → service → backend layers.
