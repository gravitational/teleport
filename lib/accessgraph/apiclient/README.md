# Access Graph Api Client

This package holds the implementation for the Access Graph client. This is used by `tctl` to make calls to access graph through the Proxy directly using a Cert with `usage:access_graph_api`.

Note that the code here is simply copied over from the Access Graph repository, which holds the spec files used to generate the client and models. This is primarily done so that Access Graph stays the source of truth (without bifurcated logic) and to avoid adding a build-time dependency on `oapi-codegen`.

## Structure

- `client.gen.go` — the generated HTTP client (copied from `access-graph`'s `pkg/api/client.gen.go`).
- `models/` — the generated request/response types, split by spec file:
  - `models/graph/` — types generated from `openapi/models/graph.yaml`.
  - `models/jsondiff/` — types generated from `openapi/models/json-diff.yaml`.
  - `models/logs/` — types generated from `openapi/models/logs.yaml`.

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
