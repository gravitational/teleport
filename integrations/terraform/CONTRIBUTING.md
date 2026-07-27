# Contributing Terraform provider resources

This document explains how to add a new Terraform resource or convert an existing legacy resource to the generic
provider driver. For the design model behind these steps, read [ARCHITECTURE.md](./ARCHITECTURE.md).

## Before you start

- Work from `integrations/terraform` unless a command says otherwise.
- Check the relevant Teleport resource API and, for RFD 153 resources, follow
  [`rfd/0153-resource-guidelines.md`](../../rfd/0153-resource-guidelines.md).
- Preserve public Terraform names, field paths, import IDs, and state behavior when converting an existing resource.
- Keep generated code generated. Do not hand-edit generated files except generated-output changes produced by the
  documented commands.

Useful commands:

```bash
# Regenerate Terraform schema/copy code and legacy generated files.
make gen-tfschema

# Build/install provider and run Terraform acceptance-style tests in testlib.
make test

# Run a focused test from the test suite.
make test TEST_ARGS='-run TestTerraformOSS/TestApp'

# Regenerate user-facing Terraform provider reference docs.
make docs
```

## Adding a new generic-driver resource

New resources should use the generic driver. The legacy generator is deprecated and will be removed after all existing
resources are converted.

### 1. Generate or update Terraform schema code

If the resource's Terraform schema/copy functions already exist in `integrations/terraform/tfschema`, reuse them.

If they do not exist:

1. Add or update a `protoc-gen-terraform-*.yaml` config file.
2. Add the relevant `protoc` invocation and `mv` step to `Makefile` target `gen-tfschema`.
3. Run:

   ```bash
   make gen-tfschema
   ```

### 2. Add a Teleport API adapter

Create `provider/internal/teleport/<resource>.go`.

The adapter should wrap `*client.Client` and implement `tfdriver.ResourceClient[T, I]` for managed resources and
`tfdriver.DataSourceClient[T, I]` for data sources.

Keep this layer focused on Teleport API calls, it should not depend on Terraform schemas, plans, state, or diagnostics.

Skeleton:

```go
package teleport

import (
    "context"

    "github.com/gravitational/trace"

    "github.com/gravitational/teleport/api/client"
    apitypes "github.com/gravitational/teleport/api/types"

    "github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
)

func NewFooClient(c *client.Client) FooClient {
    return FooClient{client: c}
}

type FooClient struct {
    client *client.Client
}

func (c FooClient) Get(ctx context.Context, id tfdriver.NameIdentifier) (*apitypes.FooV1, error) {
    foo, err := c.client.GetFoo(ctx, id.Name)
    if err != nil {
        return nil, trace.Wrap(err)
    }
    return foo, nil
}

func (c FooClient) Create(ctx context.Context, foo *apitypes.FooV1) error {
    return trace.Wrap(c.client.CreateFoo(ctx, foo))
}

func (c FooClient) Upsert(ctx context.Context, foo *apitypes.FooV1) error {
    return trace.Wrap(c.client.UpsertFoo(ctx, foo))
}

func (c FooClient) Delete(ctx context.Context, id tfdriver.NameIdentifier) error {
    return trace.Wrap(c.client.DeleteFoo(ctx, id.Name))
}
```

If the API returns an interface, type-assert it in the adapter and return a useful error on unexpected types. If update
requires preserving server-owned fields, implement `tfdriver.UpdatePreparer[T]` on the adapter.

### 3. Add resource and data source descriptors

Create `provider/internal/resources/<resource>.go`.

The descriptor wires together the API adapter, generated schema/copy functions, identifier policy, normalizers, and
revision extraction.

Skeleton:

```go
package resources

import (
    "github.com/hashicorp/terraform-plugin-framework/path"
    "github.com/hashicorp/terraform-plugin-framework/tfsdk"

    apitypes "github.com/gravitational/teleport/api/types"

    "github.com/gravitational/teleport/integrations/terraform/provider/internal/teleport"
    "github.com/gravitational/teleport/integrations/terraform/provider/internal/tfdriver"
    "github.com/gravitational/teleport/integrations/terraform/tfschema"
)

func NewFooDataSourceType() tfdriver.DataSourceType[apitypes.FooV1, tfdriver.NameIdentifier] {
    return tfdriver.DataSourceType[apitypes.FooV1, tfdriver.NameIdentifier]{
        NewDataSourceClient: func(p tfsdk.Provider) tfdriver.DataSourceClient[apitypes.FooV1, tfdriver.NameIdentifier] {
            return teleport.NewFooClient(clientFromProvider(p))
        },
        Kind: apitypes.KindFoo,
        Codec: tfdriver.DataSourceCodecFuncs[apitypes.FooV1]{
            SchemaFunc:  tfschema.GenSchemaFooV1,
            ToStateFunc: tfschema.CopyFooV1ToTerraform,
        },
        Identifier: tfdriver.NameIdentifierFromPath(path.Root("metadata").AtName("name")),
    }
}

func NewFooResourceType() tfdriver.ResourceType[apitypes.FooV1, tfdriver.NameIdentifier] {
    return tfdriver.ResourceType[apitypes.FooV1, tfdriver.NameIdentifier]{
        NewResourceClient: func(p tfsdk.Provider) tfdriver.ResourceClient[apitypes.FooV1, tfdriver.NameIdentifier] {
            return teleport.NewFooClient(clientFromProvider(p))
        },
        Kind: apitypes.KindFoo,
        Codec: tfdriver.ResourceCodecFuncs[apitypes.FooV1]{
            SchemaFunc:   tfschema.GenSchemaFooV1,
            FromPlanFunc: tfschema.CopyFooV1FromTerraform,
            ToStateFunc:  tfschema.CopyFooV1ToTerraform,
        },
        Normalizer: tfdriver.CheckAndSetDefaults[apitypes.FooV1](),
        Identifier: tfdriver.NameIdentifierPolicy(
            path.Root("metadata").AtName("name"),
            func(foo *apitypes.FooV1) string {
                return foo.GetMetadata().Name
            },
        ),
        ResourceRevision: func(foo *apitypes.FooV1) string {
            return foo.GetMetadata().Revision
        },
    }
}
```

Choose the identifier policy carefully:

- `NameIdentifierPolicy` for resources imported by name.
- `ScopeQualifiedNameIdentifierPolicy` for scoped resources.
- `CompositeIdentifierPolicy` for two-part IDs like `prefix/name`.
- `SingletonIdentifierPolicy` for singleton cluster resources.

Use `tfdriver.ForceKind` when the API type needs a kind set but Terraform should not require users to configure it.

### 4. Register the resource

Update `provider/provider.go`.

Add the resource to `genericResourceTypes` in `GetResources`:

```go
"teleport_foo": resources.NewFooResourceType(),
```

If the resource has a data source, add it to `genericDataSourceTypes` in `GetDataSources`:

```go
"teleport_foo": resources.NewFooDataSourceType(),
```

### 5. Add tests and fixtures

Add Terraform fixtures in `testlib/fixtures`, usually:

- `<resource>_0_create.tf`
- `<resource>_1_update.tf`
- `<resource>_data_source.tf`, if applicable

Add tests in `testlib/<resource>_test.go` covering:

- create, read, update, delete;
- plan stability after create and update;
- import state;
- data source behavior, if present;
- cache-enabled behavior if the resource is known to interact with cached reads.

For resources with secrets or write-only values, add checks that sensitive values do not leak into state unless the
schema intentionally stores them.

### 6. Update docs

If the resource changes the public Terraform provider surface, run:

```bash
make docs
```

See [DOCS.md](./DOCS.md) for how generated reference documentation works and how to add custom resource examples or
templates.

## Converting a legacy resource to the generic driver

The conversion path is similar to adding a new resource, but compatibility is the most important requirement.

### 1. Capture current behavior

Before editing code, inspect the existing legacy implementation in `provider/internal/legacy` and existing tests in
`testlib`.

Record the current behavior for:

- Terraform resource name and data source name
- schema paths and required/optional/computed/sensitive flags
- import ID format
- ID stored in Terraform state
- create/update method choices
- defaulting and forced kind/version behavior
- fields copied from prior state or remote state during update
- special handling for write-only or secret fields
- retry or polling behavior
- data source quirks

The generic implementation must preserve this behavior unless the change is intentional and documented.

### 2. Add the generic implementation

Add `provider/internal/teleport/<resource>.go` and `provider/internal/resources/<resource>.go` as described above.

Re-use the same generated `tfschema` schema and copy functions that the legacy resource used whenever possible. This
keeps Terraform field compatibility stable.

### 3. Switch registration

Remove the resource and data source from `provider/internal/legacy/registry.go` and add the same Terraform names to the
generic maps in `provider/provider.go`.

For example, to convert `teleport_foo`:

```go
// provider/provider.go
"teleport_foo": resources.NewFooResourceType(),
```

and, if applicable:

```go
"teleport_foo": resources.NewFooDataSourceType(),
```

Do not change the public Terraform type name during conversion.

### 4. Decide what to do with legacy generated files

Some legacy files are emitted by `gen/main.go` from payloads in that file. If a converted resource is no longer
registered, the old generated file may still compile but be unused.

Prefer removing the legacy generator payload and generated files when it is safe to do so. If the generated schema/copy
functions are still needed, keep the `protoc-gen-terraform-*.yaml` and `make gen-tfschema` entries; those are separate
from legacy provider generation.

### 5. Strengthen tests around compatibility

Run the existing resource tests unchanged first. Then add or update tests for migration-sensitive behavior:

- import an existing Teleport resource using the old import ID format;
- apply old fixture, then new fixture, and verify no unnecessary replacement;
- plan-only checks after create and update;
- data source reads with the minimal config users already rely on;
- state for sensitive/write-only fields;
- behavior when Teleport returns not found.

Use focused test runs while iterating, then run the full provider test target before submitting.

## Review checklist

Before sending a PR, verify:

- [ ] The resource is registered in exactly one active place: generic maps or legacy registry.
- [ ] Public Terraform names and import IDs are preserved for conversions.
- [ ] `id` is populated consistently for resources and data sources.
- [ ] Resource kind/version/defaults are set by Teleport defaults or explicit normalizers.
- [ ] Update waits for a real remote revision change when applicable.
- [ ] Secret and write-only fields are handled intentionally.
- [ ] Tests cover create/update/delete/import and data sources where applicable.
- [ ] `make gen-tfschema` and `make docs` were run if generated schema or public docs changed.
