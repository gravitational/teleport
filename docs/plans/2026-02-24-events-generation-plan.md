# Audit Events Generation Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Auto-generate audit event boilerplate (type constants, code constants, dynamic factory registration, test map entries, OneOf converter cases, and scaffold event proto) so new resource-gen managed resources compile immediately.

**Architecture:** Add `code_prefix` to the proto `AuditConfig` message and `spec.AuditConfig` struct. Build 5 new cross-resource gathering templates (one per output file) and 1 scaffold template for event proto messages. Wire them into `generateFiles()` in `main.go` using the same pattern as existing gathering generators.

**Tech Stack:** Go text/template, protobuf, existing resource-gen infrastructure.

---

### Task 1: Add `CodePrefix` to spec and validation

**Files:**
- Modify: `build.assets/tooling/cmd/resource-gen/spec/spec.go:76-81` (AuditConfig struct)
- Modify: `build.assets/tooling/cmd/resource-gen/spec/spec.go:105-169` (Validate function)
- Test: `build.assets/tooling/cmd/resource-gen/main_test.go`

**Step 1: Write the failing test**

Add to `main_test.go`:

```go
func TestValidateCodePrefixRequired(t *testing.T) {
	rs := spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		Storage:     spec.StorageConfig{BackendPrefix: "foo", Pattern: spec.StoragePatternStandard},
		Cache:       spec.CacheConfig{Enabled: true, Indexes: []string{"metadata.name"}},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true},
		Audit:       spec.AuditConfig{EmitOnCreate: true, CodePrefix: ""},
	}
	require.Error(t, rs.Validate(), "should fail: emit_on_create is true but code_prefix is empty")
}

func TestValidateCodePrefixFormat(t *testing.T) {
	rs := spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		Storage:     spec.StorageConfig{BackendPrefix: "foo", Pattern: spec.StoragePatternStandard},
		Cache:       spec.CacheConfig{Enabled: true, Indexes: []string{"metadata.name"}},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true},
		Audit:       spec.AuditConfig{EmitOnCreate: true, CodePrefix: "toolong"},
	}
	require.Error(t, rs.Validate(), "should fail: code_prefix must be 2-4 uppercase ASCII")
}

func TestValidateCodePrefixValid(t *testing.T) {
	rs := spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		Storage:     spec.StorageConfig{BackendPrefix: "foo", Pattern: spec.StoragePatternStandard},
		Cache:       spec.CacheConfig{Enabled: true, Indexes: []string{"metadata.name"}},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true},
		Audit:       spec.AuditConfig{EmitOnCreate: true, CodePrefix: "FO"},
	}
	require.NoError(t, rs.Validate())
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/tener/code/teleport-resource-codegen && go test ./build.assets/tooling/cmd/resource-gen/ -run TestValidateCodePrefix -v`
Expected: FAIL — `CodePrefix` field doesn't exist yet.

**Step 3: Implement spec changes**

In `spec.go`, add `CodePrefix` to `AuditConfig`:

```go
// AuditConfig controls audit event emission.
type AuditConfig struct {
	EmitOnCreate bool
	EmitOnUpdate bool
	EmitOnDelete bool
	EmitOnGet    bool
	CodePrefix   string
}
```

Add a `codePrefixPattern` regex at package level:

```go
var codePrefixPattern = regexp.MustCompile(`^[A-Z]{2,4}$`)
```

Add validation in `Validate()` after the existing audit checks (after line 166):

```go
hasAudit := s.Audit.EmitOnCreate || s.Audit.EmitOnUpdate || s.Audit.EmitOnDelete || s.Audit.EmitOnGet
if hasAudit && s.Audit.CodePrefix == "" {
	return trace.BadParameter("audit.code_prefix is required when any audit event emission is enabled")
}
if s.Audit.CodePrefix != "" && !codePrefixPattern.MatchString(s.Audit.CodePrefix) {
	return trace.BadParameter("audit.code_prefix must be 2-4 uppercase ASCII characters, got %q", s.Audit.CodePrefix)
}
```

**Step 4: Fix all existing test ResourceSpecs that have audit enabled**

Every existing `spec.ResourceSpec` in tests that sets `EmitOnCreate: true` (or any emit) now needs a `CodePrefix`. Update these:

- `main_test.go` `TestGenerateFilesCacheDisabled`: Add `Audit: spec.AuditConfig{CodePrefix: "FO"}` (this spec has `Create: true` but default audit is all-false since it's constructed directly, so no change needed — verify).
- `main_test.go` `TestGeneratedFilesHeaderAndNaming`: Same — verify if audit is set.
- `generators/generator_test.go`: Search for any `AuditConfig` usage and add `CodePrefix`.

**Step 5: Run tests to verify they pass**

Run: `cd /Users/tener/code/teleport-resource-codegen && go test ./build.assets/tooling/cmd/resource-gen/... -v`
Expected: PASS

**Step 6: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/spec/spec.go build.assets/tooling/cmd/resource-gen/main_test.go
git commit -m "resource-gen: add CodePrefix to AuditConfig with validation"
```

---

### Task 2: Add `code_prefix` to proto and parser

**Files:**
- Modify: `api/proto/teleport/options/v1/resource.proto:85-94` (AuditConfig message)
- Modify: `build.assets/tooling/cmd/resource-gen/parser/defaults.go:103-116` (audit defaults block)
- Modify: `api/proto/teleport/cookie/v1/cookie_service.proto:38-44` (cookie audit config)
- Test: `build.assets/tooling/cmd/resource-gen/main_test.go` (writeProtoFixture)

**Step 1: Add `code_prefix` to proto AuditConfig**

In `api/proto/teleport/options/v1/resource.proto`, add field 5 to `AuditConfig`:

```protobuf
// AuditConfig controls which operations emit audit events.
message AuditConfig {
  // Emit create events.
  optional bool emit_on_create = 1;
  // Emit update events.
  optional bool emit_on_update = 2;
  // Emit delete events.
  optional bool emit_on_delete = 3;
  // Emit get/read events.
  optional bool emit_on_get = 4;
  // Prefix for event code constants (2-4 uppercase ASCII chars, e.g. "CK").
  string code_prefix = 5;
}
```

**Step 2: Regenerate Go proto code**

Run: `cd /Users/tener/code/teleport-resource-codegen && make grpc/host`
Expected: Regenerated Go code for options/v1 with `GetCodePrefix()` method.

**Step 3: Extract code_prefix in parser defaults**

In `parser/defaults.go`, add to the audit config block (after `EmitOnGet` extraction, ~line 115):

```go
if cfg.Audit.GetCodePrefix() != "" {
	rs.Audit.CodePrefix = cfg.Audit.GetCodePrefix()
}
```

**Step 4: Add `code_prefix` to cookie proto**

In `api/proto/teleport/cookie/v1/cookie_service.proto`, add `code_prefix: "CK"` to the audit block:

```protobuf
audit: {
  emit_on_create: true
  emit_on_delete: true
  emit_on_update: true
  code_prefix: "CK"
}
```

**Step 5: Update writeProtoFixture in main_test.go**

The test proto fixture in `writeProtoFixture` doesn't set audit at all, so default audit has no emissions enabled, meaning CodePrefix validation won't trigger. No change needed to the fixture unless it sets any `emit_on_*` to true. Verify this.

**Step 6: Run tests**

Run: `cd /Users/tener/code/teleport-resource-codegen && go test ./build.assets/tooling/cmd/resource-gen/... -v`
Expected: PASS

**Step 7: Commit**

```bash
git add api/proto/ build.assets/tooling/cmd/resource-gen/parser/defaults.go
git commit -m "resource-gen: add code_prefix to proto AuditConfig and parser"
```

---

### Task 3: Cross-resource duplicate code_prefix validation

**Files:**
- Modify: `build.assets/tooling/cmd/resource-gen/main.go:150-227` (generateFiles function)
- Test: `build.assets/tooling/cmd/resource-gen/main_test.go`

**Step 1: Write the failing test**

Add to `main_test.go`:

```go
func TestGenerateFilesDuplicateCodePrefix(t *testing.T) {
	rs1 := spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		Storage:     spec.StorageConfig{BackendPrefix: "foo", Pattern: spec.StoragePatternStandard},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true},
		Audit:       spec.AuditConfig{EmitOnCreate: true, CodePrefix: "FO"},
	}
	rs2 := spec.ResourceSpec{
		ServiceName: "teleport.bar.v1.BarService",
		Kind:        "bar",
		Storage:     spec.StorageConfig{BackendPrefix: "bar", Pattern: spec.StoragePatternStandard},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true},
		Audit:       spec.AuditConfig{EmitOnCreate: true, CodePrefix: "FO"},
	}
	_, err := generateFiles([]spec.ResourceSpec{rs1, rs2}, "github.com/gravitational/teleport")
	require.Error(t, err)
	require.Contains(t, err.Error(), "duplicate")
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/tener/code/teleport-resource-codegen && go test ./build.assets/tooling/cmd/resource-gen/ -run TestGenerateFilesDuplicateCodePrefix -v`
Expected: FAIL — no duplicate check yet.

**Step 3: Add duplicate validation in generateFiles**

In `main.go`, add before the per-resource loop (after line 152, before `for _, rs := range specs`):

```go
// Validate cross-resource uniqueness of audit code prefixes.
prefixOwner := map[string]string{} // code_prefix -> kind
for _, rs := range specs {
	if rs.Audit.CodePrefix == "" {
		continue
	}
	if owner, exists := prefixOwner[rs.Audit.CodePrefix]; exists {
		return nil, trace.BadParameter("duplicate audit code_prefix %q: used by both %q and %q", rs.Audit.CodePrefix, owner, rs.Kind)
	}
	prefixOwner[rs.Audit.CodePrefix] = rs.Kind
}
```

**Step 4: Run tests**

Run: `cd /Users/tener/code/teleport-resource-codegen && go test ./build.assets/tooling/cmd/resource-gen/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/main.go build.assets/tooling/cmd/resource-gen/main_test.go
git commit -m "resource-gen: validate unique code_prefix across resources"
```

---

### Task 4: Event type constants gathering template and generator

**Files:**
- Create: `build.assets/tooling/cmd/resource-gen/generators/templates/events_api.go.tmpl`
- Create: `build.assets/tooling/cmd/resource-gen/generators/events_gathering.go`
- Test: `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`

This is the first of 5 cross-resource event files. It generates `lib/events/api.gen.go` containing event type string constants like `CookieCreateEvent = "resource.cookie.create"`.

**Step 1: Write the failing test**

Add to `generators/generator_test.go`:

```go
func TestGenerateEventsAPI(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:       "cookie",
			Operations: spec.OperationSet{Create: true, Update: true, Delete: true},
			Audit:      spec.AuditConfig{EmitOnCreate: true, EmitOnUpdate: true, EmitOnDelete: true, CodePrefix: "CK"},
		},
	}
	got, err := generators.GenerateEventsAPI(specs)
	require.NoError(t, err)
	require.Contains(t, got, `CookieCreateEvent = "resource.cookie.create"`)
	require.Contains(t, got, `CookieUpdateEvent = "resource.cookie.update"`)
	require.Contains(t, got, `CookieDeleteEvent = "resource.cookie.delete"`)
	require.NotContains(t, got, "GetEvent") // emit_on_get is false
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/tener/code/teleport-resource-codegen && go test ./build.assets/tooling/cmd/resource-gen/generators/ -run TestGenerateEventsAPI -v`
Expected: FAIL — function doesn't exist.

**Step 3: Create the template**

Create `build.assets/tooling/cmd/resource-gen/generators/templates/events_api.go.tmpl`:

```
package events

// Generated event type constants for resource-gen managed resources.

const (
{{- range .Events}}
	// {{.ConstName}}Event is the event type for {{.Lower}} {{.OpLower}} operations.
	{{.ConstName}}Event = "resource.{{.Lower}}.{{.OpLower}}"
{{- end}}
)
```

**Step 4: Create the generator**

Create `build.assets/tooling/cmd/resource-gen/generators/events_gathering.go`:

```go
package generators

import (
	"sort"

	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

type eventEntry struct {
	ConstName string // e.g. "CookieCreate"
	Lower     string // e.g. "cookie"
	OpLower   string // e.g. "create"
	Code      string // e.g. "CK001I"
}

type eventsAPIData struct {
	Events []eventEntry
}

var eventsAPITmpl = mustReadTemplate("events_api.go.tmpl")

// GenerateEventsAPI renders lib/events/api.gen.go with event type string
// constants for all resources that have audit events enabled.
func GenerateEventsAPI(specs []spec.ResourceSpec) (string, error) {
	entries := buildEventEntries(specs)
	if len(entries) == 0 {
		return "package events\n", nil
	}
	data := eventsAPIData{Events: entries}
	out, err := render("eventsAPI", eventsAPITmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}

func buildEventEntries(specs []spec.ResourceSpec) []eventEntry {
	var entries []eventEntry
	for _, rs := range specs {
		kind := exportedName(rs.Kind)
		lower := rs.Kind
		prefix := rs.Audit.CodePrefix

		if rs.Audit.EmitOnCreate && rs.Operations.Create {
			entries = append(entries, eventEntry{ConstName: kind + "Create", Lower: lower, OpLower: "create", Code: prefix + "001I"})
		}
		if rs.Audit.EmitOnUpdate && (rs.Operations.Update || rs.Operations.Upsert) {
			entries = append(entries, eventEntry{ConstName: kind + "Update", Lower: lower, OpLower: "update", Code: prefix + "002I"})
		}
		if rs.Audit.EmitOnDelete && rs.Operations.Delete {
			entries = append(entries, eventEntry{ConstName: kind + "Delete", Lower: lower, OpLower: "delete", Code: prefix + "003I"})
		}
		if rs.Audit.EmitOnGet && rs.Operations.Get {
			entries = append(entries, eventEntry{ConstName: kind + "Get", Lower: lower, OpLower: "get", Code: prefix + "004I"})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ConstName < entries[j].ConstName
	})
	return entries
}
```

**Step 5: Run test to verify it passes**

Run: `cd /Users/tener/code/teleport-resource-codegen && go test ./build.assets/tooling/cmd/resource-gen/generators/ -run TestGenerateEventsAPI -v`
Expected: PASS

**Step 6: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/generators/events_gathering.go build.assets/tooling/cmd/resource-gen/generators/templates/events_api.go.tmpl
git commit -m "resource-gen: add event type constants gathering generator"
```

---

### Task 5: Event code constants gathering template

**Files:**
- Create: `build.assets/tooling/cmd/resource-gen/generators/templates/events_codes.go.tmpl`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/events_gathering.go` (add GenerateEventsCodes)
- Test: `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`

**Step 1: Write the failing test**

```go
func TestGenerateEventsCodes(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:       "cookie",
			Operations: spec.OperationSet{Create: true, Update: true, Delete: true},
			Audit:      spec.AuditConfig{EmitOnCreate: true, EmitOnUpdate: true, EmitOnDelete: true, CodePrefix: "CK"},
		},
	}
	got, err := generators.GenerateEventsCodes(specs)
	require.NoError(t, err)
	require.Contains(t, got, `CookieCreateCode = "CK001I"`)
	require.Contains(t, got, `CookieUpdateCode = "CK002I"`)
	require.Contains(t, got, `CookieDeleteCode = "CK003I"`)
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL — function doesn't exist.

**Step 3: Create the template**

Create `build.assets/tooling/cmd/resource-gen/generators/templates/events_codes.go.tmpl`:

```
package events

// Generated event code constants for resource-gen managed resources.

const (
{{- range .Events}}
	// {{.ConstName}}Code is the event code for {{.Lower}} {{.OpLower}} operations.
	{{.ConstName}}Code = "{{.Code}}"
{{- end}}
)
```

**Step 4: Add GenerateEventsCodes function**

Add to `events_gathering.go`:

```go
type eventsCodesData struct {
	Events []eventEntry
}

var eventsCodesTmpl = mustReadTemplate("events_codes.go.tmpl")

// GenerateEventsCodes renders lib/events/codes.gen.go with event code
// constants for all resources that have audit events enabled.
func GenerateEventsCodes(specs []spec.ResourceSpec) (string, error) {
	entries := buildEventEntries(specs)
	if len(entries) == 0 {
		return "package events\n", nil
	}
	data := eventsCodesData{Events: entries}
	out, err := render("eventsCodes", eventsCodesTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}
```

**Step 5: Run test to verify it passes**

Expected: PASS

**Step 6: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/generators/events_gathering.go build.assets/tooling/cmd/resource-gen/generators/templates/events_codes.go.tmpl build.assets/tooling/cmd/resource-gen/generators/generator_test.go
git commit -m "resource-gen: add event code constants gathering generator"
```

---

### Task 6: Dynamic event factory gathering template

**Files:**
- Create: `build.assets/tooling/cmd/resource-gen/generators/templates/events_dynamic.go.tmpl`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/events_gathering.go` (add GenerateEventsDynamic)
- Test: `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`

This generates `lib/events/dynamic.gen.go`. It uses an `init()` that calls a `RegisterGeneratedDynamicEvents` function (defined in hand-written `dynamic.go`) to register event type strings mapped to empty proto struct instances.

**Step 1: Write the failing test**

```go
func TestGenerateEventsDynamic(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:        "cookie",
			ServiceName: "teleport.cookie.v1.CookieService",
			Operations:  spec.OperationSet{Create: true, Delete: true},
			Audit:       spec.AuditConfig{EmitOnCreate: true, EmitOnDelete: true, CodePrefix: "CK"},
		},
	}
	got, err := generators.GenerateEventsDynamic(specs, "github.com/gravitational/teleport")
	require.NoError(t, err)
	require.Contains(t, got, "CookieCreateEvent")
	require.Contains(t, got, "apievents.CookieCreate{}")
	require.Contains(t, got, "init()")
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL — function doesn't exist.

**Step 3: Create the template**

Create `build.assets/tooling/cmd/resource-gen/generators/templates/events_dynamic.go.tmpl`:

```
package events

// Generated dynamic event factory registrations for resource-gen managed resources.

import apievents "{{.Module}}/api/types/events"

func init() {
{{- range .Events}}
	RegisterGeneratedDynamicEvent({{.ConstName}}Event, func() apievents.AuditEvent { return &apievents.{{.ConstName}}{} })
{{- end}}
}
```

**Step 4: Add GenerateEventsDynamic function**

Add to `events_gathering.go`:

```go
type eventsDynamicData struct {
	Module string
	Events []eventEntry
}

var eventsDynamicTmpl = mustReadTemplate("events_dynamic.go.tmpl")

// GenerateEventsDynamic renders lib/events/dynamic.gen.go with init()
// registrations mapping event type strings to empty struct constructors.
func GenerateEventsDynamic(specs []spec.ResourceSpec, module string) (string, error) {
	entries := buildEventEntries(specs)
	if len(entries) == 0 {
		return "package events\n", nil
	}
	data := eventsDynamicData{Module: module, Events: entries}
	out, err := render("eventsDynamic", eventsDynamicTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}
```

**Step 5: Run test to verify it passes**

Expected: PASS

**Step 6: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/generators/events_gathering.go build.assets/tooling/cmd/resource-gen/generators/templates/events_dynamic.go.tmpl build.assets/tooling/cmd/resource-gen/generators/generator_test.go
git commit -m "resource-gen: add dynamic event factory gathering generator"
```

---

### Task 7: Events test map gathering template

**Files:**
- Create: `build.assets/tooling/cmd/resource-gen/generators/templates/events_test.go.tmpl`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/events_gathering.go` (add GenerateEventsTest)
- Test: `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`

This generates `lib/events/events_test.gen.go`. It uses an `init()` that calls a `RegisterGeneratedTestEvent` function to add entries to the test event coverage map.

**Step 1: Write the failing test**

```go
func TestGenerateEventsTest(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:        "cookie",
			ServiceName: "teleport.cookie.v1.CookieService",
			Operations:  spec.OperationSet{Create: true},
			Audit:       spec.AuditConfig{EmitOnCreate: true, CodePrefix: "CK"},
		},
	}
	got, err := generators.GenerateEventsTest(specs, "github.com/gravitational/teleport")
	require.NoError(t, err)
	require.Contains(t, got, "CookieCreateEvent")
	require.Contains(t, got, "CookieCreateCode")
	require.Contains(t, got, "apievents.CookieCreate{}")
	require.Contains(t, got, "init()")
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL

**Step 3: Create the template**

Create `build.assets/tooling/cmd/resource-gen/generators/templates/events_test.go.tmpl`:

```
package events

// Generated test event map entries for resource-gen managed resources.

import apievents "{{.Module}}/api/types/events"

func init() {
{{- range .Events}}
	RegisterGeneratedTestEvent(testEvent{
		eventType: {{.ConstName}}Event,
		eventCode: {{.ConstName}}Code,
		event:     &apievents.{{.ConstName}}{},
	})
{{- end}}
}
```

**Step 4: Add GenerateEventsTest function**

Add to `events_gathering.go`:

```go
type eventsTestData struct {
	Module string
	Events []eventEntry
}

var eventsTestTmpl = mustReadTemplate("events_test.go.tmpl")

// GenerateEventsTest renders lib/events/events_test.gen.go with init()
// registrations for the test event coverage map.
func GenerateEventsTest(specs []spec.ResourceSpec, module string) (string, error) {
	entries := buildEventEntries(specs)
	if len(entries) == 0 {
		return "package events\n", nil
	}
	data := eventsTestData{Module: module, Events: entries}
	out, err := render("eventsTest", eventsTestTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}
```

**Step 5: Run test to verify it passes**

Expected: PASS

**Step 6: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/generators/events_gathering.go build.assets/tooling/cmd/resource-gen/generators/templates/events_test.go.tmpl build.assets/tooling/cmd/resource-gen/generators/generator_test.go
git commit -m "resource-gen: add events test map gathering generator"
```

---

### Task 8: OneOf converter gathering template

**Files:**
- Create: `build.assets/tooling/cmd/resource-gen/generators/templates/events_oneof.go.tmpl`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/events_gathering.go` (add GenerateEventsOneOf)
- Test: `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`

This generates `api/types/events/oneof.gen.go`. It uses an `init()` that calls a `RegisterGeneratedOneOf` function to add OneOf converter cases mapping event structs to their OneOf wrapper.

**Step 1: Write the failing test**

```go
func TestGenerateEventsOneOf(t *testing.T) {
	specs := []spec.ResourceSpec{
		{
			Kind:        "cookie",
			ServiceName: "teleport.cookie.v1.CookieService",
			Operations:  spec.OperationSet{Create: true, Delete: true},
			Audit:       spec.AuditConfig{EmitOnCreate: true, EmitOnDelete: true, CodePrefix: "CK"},
		},
	}
	got, err := generators.GenerateEventsOneOf(specs)
	require.NoError(t, err)
	require.Contains(t, got, "*CookieCreate")
	require.Contains(t, got, "*CookieDelete")
	require.Contains(t, got, "init()")
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL

**Step 3: Create the template**

Create `build.assets/tooling/cmd/resource-gen/generators/templates/events_oneof.go.tmpl`:

```
package events

// Generated OneOf converter registrations for resource-gen managed resources.

func init() {
{{- range .Events}}
	RegisterGeneratedOneOf(func(e AuditEvent) isOneOf_Event {
		if evt, ok := e.(*{{.ConstName}}); ok {
			return &OneOf_{{.ConstName}}{{{.ConstName}}: evt}
		}
		return nil
	})
{{- end}}
}
```

**Step 4: Add GenerateEventsOneOf function**

Add to `events_gathering.go`:

```go
type eventsOneOfData struct {
	Events []eventEntry
}

var eventsOneOfTmpl = mustReadTemplate("events_oneof.go.tmpl")

// GenerateEventsOneOf renders api/types/events/oneof.gen.go with init()
// registrations for the ToOneOf converter.
func GenerateEventsOneOf(specs []spec.ResourceSpec) (string, error) {
	entries := buildEventEntries(specs)
	if len(entries) == 0 {
		return "package events\n", nil
	}
	data := eventsOneOfData{Events: entries}
	out, err := render("eventsOneOf", eventsOneOfTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}
```

**Step 5: Run test to verify it passes**

Expected: PASS

**Step 6: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/generators/events_gathering.go build.assets/tooling/cmd/resource-gen/generators/templates/events_oneof.go.tmpl build.assets/tooling/cmd/resource-gen/generators/generator_test.go
git commit -m "resource-gen: add OneOf converter gathering generator"
```

---

### Task 9: Scaffold event proto template and generator

**Files:**
- Create: `build.assets/tooling/cmd/resource-gen/generators/templates/events_proto_scaffold.proto.tmpl`
- Create: `build.assets/tooling/cmd/resource-gen/generators/events_scaffold.go`
- Modify: `build.assets/tooling/cmd/resource-gen/generators/templates.go` (add `*.proto.tmpl` to embed glob)
- Modify: `build.assets/tooling/cmd/resource-gen/generators/registry.go` (add scaffold generator)
- Test: `build.assets/tooling/cmd/resource-gen/generators/generator_test.go`

This is a per-resource scaffold generator (SkipIfExists: true) that generates `api/proto/teleport/events/v1/{kind}.proto` with standard embedded messages.

**Step 1: Write the failing test**

```go
func TestGenerateEventsProtoScaffold(t *testing.T) {
	rs := spec.ResourceSpec{
		Kind:        "cookie",
		ServiceName: "teleport.cookie.v1.CookieService",
		Storage:     spec.StorageConfig{BackendPrefix: "cookies", Pattern: spec.StoragePatternStandard},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true},
		Audit:       spec.AuditConfig{EmitOnCreate: true, EmitOnUpdate: true, EmitOnDelete: true, CodePrefix: "CK"},
	}
	got, err := generators.GenerateEventsProtoScaffold(rs, "github.com/gravitational/teleport")
	require.NoError(t, err)
	require.Contains(t, got, "message CookieCreate")
	require.Contains(t, got, "message CookieUpdate")
	require.Contains(t, got, "message CookieDelete")
	require.NotContains(t, got, "message CookieGet") // emit_on_get is false
	require.Contains(t, got, "events.Metadata")
	require.Contains(t, got, "events.ResourceMetadata")
}
```

**Step 2: Run test to verify it fails**

Expected: FAIL

**Step 3: Update embed glob**

In `templates.go`, change:

```go
//go:embed templates/*.go.tmpl
```

to:

```go
//go:embed templates/*.go.tmpl templates/*.proto.tmpl
```

**Step 4: Create the proto scaffold template**

Create `build.assets/tooling/cmd/resource-gen/generators/templates/events_proto_scaffold.proto.tmpl`:

```
// Copyright 2026 Gravitational, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

syntax = "proto3";

package teleport.events.v1;

import "teleport/legacy/types/events/events.proto";

option go_package = "github.com/gravitational/teleport/api/types/events;apievents";
{{range .Messages}}
// {{.Name}} is emitted when a {{.Lower}} resource is {{.OpPastTense}}.
// TODO: Add resource-specific fields after the standard embeds.
// TODO: Add this message to the OneOf in api/proto/teleport/legacy/types/events/events.proto.
message {{.Name}} {
  events.Metadata Metadata = 1
      [(gogoproto.nullable) = false, (gogoproto.embed) = true, (gogoproto.jsontag) = ""];
  events.Status Status = 2
      [(gogoproto.nullable) = false, (gogoproto.embed) = true, (gogoproto.jsontag) = ""];
  events.UserMetadata User = 3
      [(gogoproto.nullable) = false, (gogoproto.embed) = true, (gogoproto.jsontag) = ""];
  events.ConnectionMetadata Connection = 4
      [(gogoproto.nullable) = false, (gogoproto.embed) = true, (gogoproto.jsontag) = ""];
  events.ResourceMetadata Resource = 5
      [(gogoproto.nullable) = false, (gogoproto.embed) = true, (gogoproto.jsontag) = ""];
}
{{end}}
```

**Step 5: Create the scaffold generator**

Create `build.assets/tooling/cmd/resource-gen/generators/events_scaffold.go`:

```go
package generators

import (
	"github.com/gravitational/teleport/build.assets/tooling/cmd/resource-gen/spec"
	"github.com/gravitational/trace"
)

type eventMessage struct {
	Name         string // e.g. "CookieCreate"
	Lower        string // e.g. "cookie"
	OpPastTense  string // e.g. "created"
}

type eventsProtoScaffoldData struct {
	Messages []eventMessage
}

var eventsProtoScaffoldTmpl = mustReadTemplate("events_proto_scaffold.proto.tmpl")

// GenerateEventsProtoScaffold renders a scaffold proto file for event messages.
// This file is created once per resource and never overwritten.
func GenerateEventsProtoScaffold(rs spec.ResourceSpec, module string) (string, error) {
	kind := exportedName(rs.Kind)
	lower := rs.Kind

	var msgs []eventMessage
	if rs.Audit.EmitOnCreate && rs.Operations.Create {
		msgs = append(msgs, eventMessage{Name: kind + "Create", Lower: lower, OpPastTense: "created"})
	}
	if rs.Audit.EmitOnUpdate && (rs.Operations.Update || rs.Operations.Upsert) {
		msgs = append(msgs, eventMessage{Name: kind + "Update", Lower: lower, OpPastTense: "updated"})
	}
	if rs.Audit.EmitOnDelete && rs.Operations.Delete {
		msgs = append(msgs, eventMessage{Name: kind + "Delete", Lower: lower, OpPastTense: "deleted"})
	}
	if rs.Audit.EmitOnGet && rs.Operations.Get {
		msgs = append(msgs, eventMessage{Name: kind + "Get", Lower: lower, OpPastTense: "read"})
	}

	if len(msgs) == 0 {
		return "", nil
	}

	data := eventsProtoScaffoldData{Messages: msgs}
	out, err := render("eventsProtoScaffold", eventsProtoScaffoldTmpl, data)
	if err != nil {
		return "", trace.Wrap(err)
	}
	return out, nil
}
```

**Step 6: Add the scaffold generator to the registry**

In `registry.go`, add to the `Generators()` slice (after the `tctl-registration` entry):

```go
{
	Name: "events-proto-scaffold",
	PathFunc: func(kind string, _ spec.ResourceSpec) string {
		return filepath.Join("api", "proto", "teleport", "events", "v1", kind+".proto")
	},
	Generate:     GenerateEventsProtoScaffold,
	SkipIfExists: true,
	Condition:    func(rs spec.ResourceSpec) bool { return rs.Audit.CodePrefix != "" },
},
```

**Step 7: Run tests**

Run: `cd /Users/tener/code/teleport-resource-codegen && go test ./build.assets/tooling/cmd/resource-gen/... -v`
Expected: PASS

**Step 8: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/generators/
git commit -m "resource-gen: add scaffold event proto generator"
```

---

### Task 10: Wire cross-resource event generators into main.go

**Files:**
- Modify: `build.assets/tooling/cmd/resource-gen/main.go:150-227` (generateFiles function)
- Modify: `build.assets/tooling/cmd/resource-gen/main_test.go` (assertions)

**Step 1: Write the failing test**

Update `TestRunWithWriterDryRun` to add expected event output files:

```go
// Add after existing assertions in TestRunWithWriterDryRun:
require.Contains(t, out.String(), "lib/events/api.gen.go")
require.Contains(t, out.String(), "lib/events/codes.gen.go")
require.Contains(t, out.String(), "lib/events/dynamic.gen.go")
require.Contains(t, out.String(), "lib/events/events_test.gen.go")
require.Contains(t, out.String(), "api/types/events/oneof.gen.go")
```

Note: The fixture proto needs `code_prefix` and audit to produce event files. If the fixture has no audit config, these cross-resource files might be empty or skipped. Either update the fixture to include `code_prefix` or conditionally check. For simplicity, add `code_prefix: "FO"` and audit fields to the fixture service proto. Update `writeProtoFixture` — add to the `resource_config` in the service proto:

```
audit: {
  emit_on_create: true
  code_prefix: "FO"
}
```

Also update the proto options message in the fixture to include the `AuditConfig`:

```
message AuditConfig {
  optional bool emit_on_create = 1;
  optional bool emit_on_update = 2;
  optional bool emit_on_delete = 3;
  optional bool emit_on_get = 4;
  string code_prefix = 5;
}
```

And add `AuditConfig audit = 4;` to `ResourceConfig`.

**Step 2: Run test to verify it fails**

Expected: FAIL — event files not generated yet.

**Step 3: Add cross-resource event blocks to generateFiles**

In `main.go`, add 5 new blocks after the authclient gathering block, following the exact same pattern:

```go
// Cross-resource: event type constants (lib/events/api.gen.go)
if len(specs) > 0 {
	content, err := generators.GenerateEventsAPI(specs)
	if err != nil {
		return nil, trace.Wrap(err, "generating event type constants")
	}
	files = append(files, generatedFile{
		Path:    filepath.Join("lib", "events", "api.gen.go"),
		Content: generatedHeader + content,
	})
}

// Cross-resource: event code constants (lib/events/codes.gen.go)
if len(specs) > 0 {
	content, err := generators.GenerateEventsCodes(specs)
	if err != nil {
		return nil, trace.Wrap(err, "generating event code constants")
	}
	files = append(files, generatedFile{
		Path:    filepath.Join("lib", "events", "codes.gen.go"),
		Content: generatedHeader + content,
	})
}

// Cross-resource: dynamic event factory (lib/events/dynamic.gen.go)
if len(specs) > 0 {
	content, err := generators.GenerateEventsDynamic(specs, module)
	if err != nil {
		return nil, trace.Wrap(err, "generating dynamic event factory")
	}
	files = append(files, generatedFile{
		Path:    filepath.Join("lib", "events", "dynamic.gen.go"),
		Content: generatedHeader + content,
	})
}

// Cross-resource: events test map (lib/events/events_test.gen.go)
if len(specs) > 0 {
	content, err := generators.GenerateEventsTest(specs, module)
	if err != nil {
		return nil, trace.Wrap(err, "generating events test map")
	}
	files = append(files, generatedFile{
		Path:    filepath.Join("lib", "events", "events_test.gen.go"),
		Content: generatedHeader + content,
	})
}

// Cross-resource: OneOf converter (api/types/events/oneof.gen.go)
if len(specs) > 0 {
	content, err := generators.GenerateEventsOneOf(specs)
	if err != nil {
		return nil, trace.Wrap(err, "generating OneOf converter")
	}
	files = append(files, generatedFile{
		Path:    filepath.Join("api", "types", "events", "oneof.gen.go"),
		Content: generatedHeader + content,
	})
}
```

**Step 4: Run tests**

Run: `cd /Users/tener/code/teleport-resource-codegen && go test ./build.assets/tooling/cmd/resource-gen/... -v`
Expected: PASS

**Step 5: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/main.go build.assets/tooling/cmd/resource-gen/main_test.go
git commit -m "resource-gen: wire event gathering generators into main pipeline"
```

---

### Task 11: Verify header/naming invariants and end-to-end test

**Files:**
- Modify: `build.assets/tooling/cmd/resource-gen/main_test.go`

The existing `TestGeneratedFilesHeaderAndNaming` test already checks that:
- Generated files (SkipIfExists=false) have the generated header AND `.gen.go` suffix
- Scaffold files (SkipIfExists=true) have neither

This test needs to be updated so the test ResourceSpec has a `CodePrefix` if audit is enabled, ensuring the new event files are included in the check.

**Step 1: Update TestGeneratedFilesHeaderAndNaming**

If the ResourceSpec in this test sets any `EmitOn*` to true, add `CodePrefix: "FO"`. If not (all audit fields are default false since it's constructed directly without defaults), no change needed. Verify and adjust.

Also add a dedicated test:

```go
func TestGeneratedEventsFilesHeaderAndNaming(t *testing.T) {
	rs := spec.ResourceSpec{
		ServiceName: "teleport.foo.v1.FooService",
		Kind:        "foo",
		Storage:     spec.StorageConfig{BackendPrefix: "foo", Pattern: spec.StoragePatternStandard},
		Cache:       spec.CacheConfig{Enabled: true, Indexes: []string{"metadata.name"}},
		Pagination:  spec.PaginationConfig{DefaultPageSize: 200, MaxPageSize: 1000},
		Operations:  spec.OperationSet{Get: true, List: true, Create: true, Update: true, Delete: true},
		Audit:       spec.AuditConfig{EmitOnCreate: true, EmitOnUpdate: true, EmitOnDelete: true, CodePrefix: "FO"},
	}

	files, err := generateFiles([]spec.ResourceSpec{rs}, "github.com/gravitational/teleport")
	require.NoError(t, err)

	eventPaths := []string{
		filepath.Join("lib", "events", "api.gen.go"),
		filepath.Join("lib", "events", "codes.gen.go"),
		filepath.Join("lib", "events", "dynamic.gen.go"),
		filepath.Join("lib", "events", "events_test.gen.go"),
		filepath.Join("api", "types", "events", "oneof.gen.go"),
	}

	for _, ep := range eventPaths {
		found := false
		for _, f := range files {
			if f.Path == ep {
				found = true
				require.True(t, strings.HasPrefix(f.Content, generatedHeader),
					"event file %s must have generated header", ep)
				require.True(t, strings.HasSuffix(f.Path, ".gen.go"),
					"event file %s must have .gen.go suffix", ep)
				require.False(t, f.SkipIfExists,
					"event gathering file %s must not be a scaffold", ep)
				break
			}
		}
		require.True(t, found, "expected event file %s to be generated", ep)
	}

	// Scaffold event proto must NOT have header or .gen.go suffix
	scaffoldPath := filepath.Join("api", "proto", "teleport", "events", "v1", "foo.proto")
	for _, f := range files {
		if f.Path == scaffoldPath {
			require.True(t, f.SkipIfExists, "event proto scaffold must be SkipIfExists")
			require.False(t, strings.HasPrefix(f.Content, generatedHeader),
				"event proto scaffold must not have generated header")
			require.False(t, strings.HasSuffix(f.Path, ".gen.go"),
				"event proto scaffold must not have .gen.go suffix")
			break
		}
	}
}
```

**Step 2: Run tests**

Run: `cd /Users/tener/code/teleport-resource-codegen && go test ./build.assets/tooling/cmd/resource-gen/... -v`
Expected: PASS

**Step 3: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/main_test.go
git commit -m "resource-gen: add end-to-end tests for event generation files"
```

---

### Task 12: Update README and docs

**Files:**
- Modify: `build.assets/tooling/cmd/resource-gen/README.md`
- Modify: `docs/resource-codegen/new-resource.md`
- Modify: `build.assets/tooling/cmd/resource-gen/TODO.md`

**Step 1: Update README.md**

Add `code_prefix` to the proto option reference section. Add the 5 new cross-resource files and 1 scaffold file to the generated files table.

**Step 2: Update new-resource.md**

- Step 2 now also generates event proto scaffold and all event registration files
- Add a new sub-step about adding OneOf entries to `events.proto`
- Note that event type constants and codes are auto-generated

**Step 3: Update TODO.md**

Mark item #3 as done: `~~Generate events stuff: types + constants.~~ Done: generates event type/code constants, dynamic factory, test map, OneOf converter, and scaffold event proto.`

**Step 4: Commit**

```bash
git add build.assets/tooling/cmd/resource-gen/README.md docs/resource-codegen/new-resource.md build.assets/tooling/cmd/resource-gen/TODO.md
git commit -m "docs: update for events generation support"
```
