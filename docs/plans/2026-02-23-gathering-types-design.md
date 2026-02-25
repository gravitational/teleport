# Gathering Type Generation: Design Document

Date: 2026-02-23

## Problem

Adding a new generated resource currently requires manual edits to two "gathering" types:

1. `lib/auth/services.gen.go` — `servicesGenerated` struct with a named field per resource and a constructor that initializes each from the backend
2. `lib/cache/index.gen.go` — `generatedConfig` struct that embeds each resource's service interface

Both files already have `.gen.go` suffixes and comments indicating they should be generated. The generator's cross-resource pipeline (used by `GenerateKindConstants`) supports this pattern.

## Design

Two new cross-resource generators, each with its own template, following the `GenerateKindConstants` pattern.

### Generator 1: `GenerateServicesGathering`

**Signature:** `GenerateServicesGathering(specs []spec.ResourceSpec, module string) (string, error)`

**Output path:** `lib/auth/services.gen.go`

**Produces:**

```go
package auth

import (
    "github.com/gravitational/trace"
    "github.com/gravitational/teleport/lib/services"
    "github.com/gravitational/teleport/lib/services/local"
)

type servicesGenerated struct {
    Cookies services.Cookies
    // one field per generated resource, sorted alphabetically
}

func newServicesGenerated(cfg *InitConfig) (*servicesGenerated, error) {
    var err error
    gen := &servicesGenerated{}

    gen.Cookies, err = local.NewCookieService(cfg.Backend)
    if err != nil {
        return nil, trace.Wrap(err)
    }
    // one block per resource

    return gen, nil
}
```

All generated resources are included (the `Services` struct aggregates all backend services).

### Generator 2: `GenerateCacheGathering`

**Signature:** `GenerateCacheGathering(specs []spec.ResourceSpec, module string) (string, error)`

**Output path:** `lib/cache/index.gen.go`

**Produces:**

```go
package cache

import "github.com/gravitational/teleport/lib/services"

// generatedConfig is regenerated in full on each resource-gen run.
type generatedConfig struct {
    services.Cookies
    // one embedded interface per cache-enabled resource, sorted alphabetically
}
```

Only resources with `Cache.Enabled == true` are included, matching the existing pattern where cache registration is conditional.

### Template data

Both generators use a shared entry type with fields needed for the templates: `Kind` (PascalCase), `Lower` (lowercase), `Plural` (e.g., "Cookies"). The services gathering also needs the `Kind` name for the constructor call pattern (`local.New<Kind>Service`).

### Pipeline wiring

Both generators are added to `generateFiles()` in `main.go` after the existing `GenerateKindConstants` call, guarded by `len(specs) > 0`.

### Compile-time check

The existing compile-time check in `cache_registration.go.tmpl` (`var _ services.Cookies = Config{}.Cookies`) is kept as a safety net.

### Testing

Both generators get unit tests in `generator_test.go`:
- Single resource case
- Multiple resources case (verifying alphabetical ordering)
- Cache gathering only includes cache-enabled resources

## Files Changed

New files:
- `build.assets/tooling/cmd/resource-gen/generators/templates/services_gathering.go.tmpl`
- `build.assets/tooling/cmd/resource-gen/generators/templates/cache_gathering.go.tmpl`
- `build.assets/tooling/cmd/resource-gen/generators/services_gathering.go`
- `build.assets/tooling/cmd/resource-gen/generators/cache_gathering.go`

Modified files:
- `build.assets/tooling/cmd/resource-gen/main.go` — add cross-resource generation calls
- `build.assets/tooling/cmd/resource-gen/generators/generator_test.go` — add tests
