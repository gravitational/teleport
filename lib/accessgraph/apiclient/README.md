# Access Graph Api Client

This package holds the implementation for the Access Graph client. This is used by `tctl` to make calls to access graph through the Proxy directly using a Cert with `usage:access_graph_api`.

Note that the code here is simply copied over from the Access Graph repository, which holds the spec files used to generate the client and models. This is primarily done so that Access Graph stays the source of truth (without bifurcated logic) and to avoid adding a build-time dependency on `oapi-codegen`.

## Structure

- `client.gen.go` — the generated HTTP client (copied from `access-graph`'s `pkg/api/client.gen.go`).
- `models/` — the generated request/response types, split by spec file:
  - `models/graph/` — types generated from `openapi/models/graph.yaml`.
  - `models/jsondiff/` — types generated from `openapi/models/json-diff.yaml`.
  - `models/logs/` — types generated from `openapi/models/logs.yaml`.
- `internal/oapiruntime/` — a **minimal subset** of
  [`github.com/oapi-codegen/runtime`](https://github.com/oapi-codegen/oapi-codegen)
  vendored locally so Teleport does not depend on that module. See the next
  section for the supported/unsupported matrix.

### Vendored `oapi-codegen` runtime (subset)

The generated code only calls two upstream runtime entry points (`JSONMerge`
for union marshalling and `StyleParamWithOptions` for URL parameter
formatting). Instead of depending on the full module, we vendor just those
pieces in [`internal/oapiruntime`](./internal/oapiruntime). That subset
intentionally does **not** support every input shape upstream handles —
keeping it small makes it easier to audit, review, and maintain. The reference
implementation lives at
<https://github.com/oapi-codegen/oapi-codegen/tree/runtime/v1.4.0/runtime>;
anything below that's listed as "not supported" has a working implementation
there that can be ported over when we need it.

Inputs that this subset does **not** support (they return an error from
`StyleParamWithOptions` today):

- **Styles**: `deepObject`, `spaceDelimited`, `pipeDelimited`.
- **Kinds**: `reflect.Slice` (including `[]byte` with `format: "byte"`),
  `reflect.Map`, and generic `reflect.Struct` values (only `time.Time` and
  `uuid.UUID`, plus their named aliases, are accepted).
- **Formats / types**: `github.com/oapi-codegen/runtime/types.Date`
  (date-only values). The upstream `types` subpackage is not vendored at all.

If the access-graph spec grows to use any of the above, the generated
`StyleParamWithOptions` call sites will start failing at runtime and the
error-boundary tests in `internal/oapiruntime/styleparam_test.go`
(`TestStyleParamUnsupported`) will catch it in CI. When that happens, port
the matching code from the upstream reference into
`internal/oapiruntime/styleparam.go` and extend the positive tests.

### Why only `client.gen.go` and `models.gen.go`?

`access-graph` generates four kinds of files:

- `api.gen.go` — the server implementation (not needed here).
- `client.gen.go` — the client we consume.
- `models/<pkg>/models.gen.go` — the request/response types.
- `models/<pkg>/spec.gen.go` — the embedded OpenAPI spec, used server-side for Swagger handling.

The server generation and embedded-spec files pull in a large dependency tree (swagger/kin-openapi, etc.) that the client doesn't need. `access-graph` splits generation across multiple configs specifically so the client can be copied here without dragging those dependencies into teleport. Only copy `client.gen.go` and the `models.gen.go` files — **never** copy `api.gen.go` or the `spec.gen.go` files.

## How to update

When the Access Graph API changes (or when new endpoints/models are added), regenerate the code in [`access-graph`](https://github.com/gravitational/access-graph) and copy the output over. Steps:

1. **Make the API change in `access-graph`** (skip if already merged). The relevant spec files live in:
   - `openapi.yaml` — top-level spec (paths, operations).
   - `openapi/models/graph.yaml`, `openapi/models/json-diff.yaml`, `openapi/models/logs.yaml` — model definitions.

2. **Regenerate the client and models in `access-graph`** (skip if already done):

   ```sh
   make go-generate
   ```

   This produces `pkg/api/client.gen.go`, `pkg/api/api.gen.go`, and the per-model packages under `pkg/api/models/` (each with both `models.gen.go` and `spec.gen.go`).

3. **Copy the generated code into this repository**. Only copy the client and the `models.gen.go` files — skip `api.gen.go` and all `spec.gen.go` files:
   - `access-graph/pkg/api/client.gen.go` → `lib/accessgraph/apiclient/client.gen.go`.
   - `access-graph/pkg/api/models/graph/models.gen.go` → `lib/accessgraph/apiclient/models/graph/models.gen.go`.
   - `access-graph/pkg/api/models/jsondiff/models.gen.go` → `lib/accessgraph/apiclient/models/jsondiff/models.gen.go`.
   - `access-graph/pkg/api/models/logs/models.gen.go` → `lib/accessgraph/apiclient/models/logs/models.gen.go`.

4. **Fix up `client.gen.go`** so it compiles in this repo:
   - Change the package declaration from `package api` to `package accessgraph`.
   - Replace the model imports (which point back into `access-graph`) with the local paths:

     ```go
     externalRef0 "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/graph"
     externalRef1 "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/jsondiff"
     externalRef2 "github.com/gravitational/teleport/lib/accessgraph/apiclient/models/logs"
     ```

5. **Repoint the `oapi-codegen` runtime import** in `client.gen.go` and each
   `models/*/models.gen.go` that imports it. Teleport vendors the small subset
   of that runtime we actually use under
   [`internal/oapiruntime`](./internal/oapiruntime) to avoid pulling
   `github.com/oapi-codegen/runtime` (and its dependency tree) into the module.
   Replace:

   ```go
   "github.com/oapi-codegen/runtime"
   ```

   with:

   ```go
   runtime "github.com/gravitational/teleport/lib/accessgraph/apiclient/internal/oapiruntime"
   ```

   If upstream `oapi-codegen` starts emitting calls to runtime helpers beyond
   `StyleParamWithOptions` / `JSONMerge` (for example deep-object styling, byte
   slices, or `types.Date`), port those pieces back from upstream into
   `internal/oapiruntime` — see that package's README for details.

6. **Re-check the vendored runtime subset**. Scan the regenerated
   `client.gen.go` and `models/*/models.gen.go` for new parameter types and
   confirm none of them fall into the "not supported" list in the "Vendored
   `oapi-codegen` runtime (subset)" section above (new slice / map / generic
   struct parameters, `[]byte` with `format: "byte"`, `types.Date` fields, or
   any of the `deepObject` / `spaceDelimited` / `pipeDelimited` styles). If a
   regen introduces one of those, port the corresponding logic from the
   [upstream reference](https://github.com/oapi-codegen/oapi-codegen/tree/runtime/v1.4.0/runtime)
   into `internal/oapiruntime/styleparam.go` and extend
   `styleparam_test.go` to cover the new shape before shipping the update.
