# Gathering Type Generation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Generate the `servicesGenerated` and `generatedConfig` gathering types so adding a new resource requires zero manual edits to these aggregation structs.

**Architecture:** Two new cross-resource generators (`GenerateServicesGathering`, `GenerateCacheGathering`) follow the established `GenerateKindConstants` pattern. Each takes `[]spec.ResourceSpec` and `module string`, renders a template, and produces a single file. Both are wired into `main.go`'s `generateFiles()` after the existing kind constants call.

**Tech Stack:** Go, text/template, testify

**Test command:** `cd build.assets/tooling && go test ./cmd/resource-gen/...`

**Design doc:** `docs/plans/2026-02-23-gathering-types-design.md`

---

### Task 1: Generate `servicesGenerated` (`lib/auth/services.gen.go`)

**Files:**
- Create: `build.assets/tooling/cmd/resource-gen/generators/templates/services_gathering.go.tmpl`
- Create: `build.assets/tooling/cmd/resource-gen/generators/services_gathering.go`
- Modify: `build.assets/tooling/cmd/resource-gen/main.go:172-182`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`

**Step 1: Write failing tests**

In `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`, add:

```go
func TestGenerateServicesGathering(t *testing.T) {
	specs := []spec.ResourceSpec{
		testSpec(spec.OperationSet{Get: true, List: true, Create: true}),
	}

	got, err := GenerateServicesGathering(specs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package auth")
	require.Contains(t, got, "type servicesGenerated struct")
	require.Contains(t, got, "Foos services.Foos")
	require.Contains(t, got, "func newServicesGenerated(cfg *InitConfig)")
	require.Contains(t, got, "gen.Foos, err = local.NewFooService(cfg.Backend)")
	require.Contains(t, got, "trace.Wrap(err)")
}

func TestGenerateServicesGatheringMultiple(t *testing.T) {
	spec1 := testSpec(spec.OperationSet{Get: true, List: true})
	spec2 := testSpec(spec.OperationSet{Get: true, Create: true})
	spec2.Kind = "gadget"
	spec2.ServiceName = "teleport.gadget.v1.GadgetService"
	specs := []spec.ResourceSpec{spec1, spec2}

	got, err := GenerateServicesGathering(specs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "Foos services.Foos")
	require.Contains(t, got, "Gadgets services.Gadgets")
	require.Contains(t, got, "gen.Foos, err = local.NewFooService(cfg.Backend)")
	require.Contains(t, got, "gen.Gadgets, err = local.NewGadgetService(cfg.Backend)")
}
```

**Step 2: Run tests — verify they fail**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/generators/ -run TestGenerateServicesGathering -v`
Expected: FAIL — `GenerateServicesGathering` doesn't exist

**Step 3: Create the template**

Create `build.assets/tooling/cmd/resource-gen/generators/templates/services_gathering.go.tmpl`:

```
package auth

import (
	"github.com/gravitational/trace"

	"{{.Module}}/lib/services"
	"{{.Module}}/lib/services/local"
)

type servicesGenerated struct {
{{- range .Resources}}
	{{.Plural}} services.{{.Plural}}
{{- end}}
}

func newServicesGenerated(cfg *InitConfig) (*servicesGenerated, error) {
	var err error
	gen := &servicesGenerated{}
{{range .Resources}}
	gen.{{.Plural}}, err = local.New{{.Kind}}Service(cfg.Backend)
	if err != nil {
		return nil, trace.Wrap(err)
	}
{{end}}
	return gen, nil
}
```

**Step 4: Create the generator**

Create `build.assets/tooling/cmd/resource-gen/generators/services_gathering.go`:

```go
package generators

import (
	"sort"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

type gatheringEntry struct {
	Kind   string // PascalCase, e.g. "Foo"
	Lower  string // lowercase, e.g. "foo"
	Plural string // e.g. "Foos"
}

type servicesGatheringData struct {
	Module    string
	Resources []gatheringEntry
}

var servicesGatheringTmpl = mustReadTemplate("services_gathering.go.tmpl")

// GenerateServicesGathering renders lib/auth/services.gen.go with the
// servicesGenerated struct and its constructor for all generated resources.
func GenerateServicesGathering(specs []spec.ResourceSpec, module string) (string, error) {
	entries := buildGatheringEntries(specs)
	data := servicesGatheringData{
		Module:    module,
		Resources: entries,
	}
	out, err := render("servicesGathering", servicesGatheringTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}

func buildGatheringEntries(specs []spec.ResourceSpec) []gatheringEntry {
	entries := make([]gatheringEntry, 0, len(specs))
	for _, rs := range specs {
		kind := exportedName(rs.Kind)
		entries = append(entries, gatheringEntry{
			Kind:   kind,
			Lower:  rs.Kind,
			Plural: pluralize(kind),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Lower < entries[j].Lower
	})
	return entries
}
```

**Step 5: Run tests — verify they pass**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/generators/ -run TestGenerateServicesGathering -v`
Expected: PASS

**Step 6: Wire into main.go**

In `build.assets/tooling/cmd/resource-gen/main.go`, after the kind constants block (after line 182), add:

```go
	// Cross-resource: services gathering (lib/auth/services.gen.go)
	if len(specs) > 0 {
		content, err := generators.GenerateServicesGathering(specs, module)
		if err != nil {
			return nil, trace.Wrap(err, "generating services gathering")
		}
		files = append(files, generatedFile{
			Path:    filepath.Join("lib", "auth", "services.gen.go"),
			Content: content,
		})
	}
```

**Step 7: Run all tests**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/...`
Expected: All pass

**Step 8: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/
git commit -m "resource-gen: generate servicesGenerated gathering type

New cross-resource generator produces lib/auth/services.gen.go with the
servicesGenerated struct and newServicesGenerated constructor. Adding a
new resource no longer requires manually editing this file."
```

---

### Task 2: Generate `generatedConfig` (`lib/cache/index.gen.go`)

**Files:**
- Create: `build.assets/tooling/cmd/resource-gen/generators/templates/cache_gathering.go.tmpl`
- Create: `build.assets/tooling/cmd/resource-gen/generators/cache_gathering.go`
- Modify: `build.assets/tooling/cmd/resource-gen/main.go`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`

**Step 1: Write failing tests**

In `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`, add:

```go
func TestGenerateCacheGathering(t *testing.T) {
	specs := []spec.ResourceSpec{
		testSpec(spec.OperationSet{Get: true, List: true, Create: true}),
	}
	specs[0].Cache.Enabled = true

	got, err := GenerateCacheGathering(specs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "package cache")
	require.Contains(t, got, "type generatedConfig struct")
	require.Contains(t, got, "services.Foos")
}

func TestGenerateCacheGatheringExcludesDisabledCache(t *testing.T) {
	spec1 := testSpec(spec.OperationSet{Get: true, List: true})
	spec1.Cache.Enabled = true
	spec2 := testSpec(spec.OperationSet{Get: true})
	spec2.Kind = "gadget"
	spec2.ServiceName = "teleport.gadget.v1.GadgetService"
	spec2.Cache.Enabled = false
	specs := []spec.ResourceSpec{spec1, spec2}

	got, err := GenerateCacheGathering(specs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "services.Foos")
	require.NotContains(t, got, "Gadgets")
}

func TestGenerateCacheGatheringMultiple(t *testing.T) {
	spec1 := testSpec(spec.OperationSet{Get: true, List: true})
	spec1.Cache.Enabled = true
	spec2 := testSpec(spec.OperationSet{Get: true, List: true})
	spec2.Kind = "gadget"
	spec2.ServiceName = "teleport.gadget.v1.GadgetService"
	spec2.Cache.Enabled = true
	specs := []spec.ResourceSpec{spec1, spec2}

	got, err := GenerateCacheGathering(specs, testModule)
	require.NoError(t, err)
	require.Contains(t, got, "services.Foos")
	require.Contains(t, got, "services.Gadgets")
}
```

**Step 2: Run tests — verify they fail**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/generators/ -run TestGenerateCacheGathering -v`
Expected: FAIL — `GenerateCacheGathering` doesn't exist

**Step 3: Create the template**

Create `build.assets/tooling/cmd/resource-gen/generators/templates/cache_gathering.go.tmpl`:

```
package cache

import "{{.Module}}/lib/services"

// generatedConfig is regenerated in full on each resource-gen run.
type generatedConfig struct {
{{- range .Resources}}
	services.{{.Plural}}
{{- end}}
}
```

**Step 4: Create the generator**

Create `build.assets/tooling/cmd/resource-gen/generators/cache_gathering.go`:

```go
package generators

import (
	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

type cacheGatheringData struct {
	Module    string
	Resources []gatheringEntry
}

var cacheGatheringTmpl = mustReadTemplate("cache_gathering.go.tmpl")

// GenerateCacheGathering renders lib/cache/index.gen.go with the
// generatedConfig struct for all cache-enabled generated resources.
func GenerateCacheGathering(specs []spec.ResourceSpec, module string) (string, error) {
	// Filter to cache-enabled resources only.
	var cacheSpecs []spec.ResourceSpec
	for _, rs := range specs {
		if rs.Cache.Enabled {
			cacheSpecs = append(cacheSpecs, rs)
		}
	}
	entries := buildGatheringEntries(cacheSpecs)
	data := cacheGatheringData{
		Module:    module,
		Resources: entries,
	}
	out, err := render("cacheGathering", cacheGatheringTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}
```

Note: `buildGatheringEntries` is reused from `services_gathering.go` (Task 1). The `gatheringEntry` type is also shared.

**Step 5: Run tests — verify they pass**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/generators/ -run TestGenerateCacheGathering -v`
Expected: PASS

**Step 6: Wire into main.go**

In `build.assets/tooling/cmd/resource-gen/main.go`, after the services gathering block added in Task 1, add:

```go
	// Cross-resource: cache gathering (lib/cache/index.gen.go)
	if len(specs) > 0 {
		content, err := generators.GenerateCacheGathering(specs, module)
		if err != nil {
			return nil, trace.Wrap(err, "generating cache gathering")
		}
		files = append(files, generatedFile{
			Path:    filepath.Join("lib", "cache", "index.gen.go"),
			Content: content,
		})
	}
```

**Step 7: Run all tests**

Run: `cd build.assets/tooling && go test ./cmd/resource-gen/...`
Expected: All pass

**Step 8: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/
git commit -m "resource-gen: generate generatedConfig cache gathering type

New cross-resource generator produces lib/cache/index.gen.go with the
generatedConfig struct. Only cache-enabled resources are included.
Adding a new resource no longer requires manually editing this file."
```
