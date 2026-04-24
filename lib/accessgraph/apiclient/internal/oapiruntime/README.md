# oapiruntime

This package vendors the small slice of
[`github.com/oapi-codegen/runtime`](https://github.com/oapi-codegen/oapi-codegen)
v1.4.0 that the access-graph client needs, so that Teleport does not have to
depend on the upstream module. The upstream code is Apache License 2.0; a copy
lives in [`LICENSE`](./LICENSE).

`JSONMerge` upstream in turn depends on
[`github.com/apapsch/go-jsonmerge/v2`](https://github.com/apapsch/go-jsonmerge)
v2.0.0 (MIT). To drop that second dependency as well, the merge logic is
inlined in `merge.go`; a copy of its MIT notice lives in
[`LICENSE-jsonmerge`](./LICENSE-jsonmerge).

The access-graph `oapi-codegen`-generated code only calls two upstream
entry points:

- `JSONMerge` — used by `oneOf` / `anyOf` union marshalling in the generated
  models.
- `StyleParamWithOptions` (and its `StyleParamOptions`, `ParamLocation*`
  siblings) — used by the generated client to format path and query
  parameters.

## What's included

| File                   | Relationship to upstream                                                                 |
| ---------------------- | ---------------------------------------------------------------------------------------- |
| `jsonmerge.go`         | Adapted from `oapi-codegen/runtime` `jsonmerge.go`: only the external `github.com/apapsch/go-jsonmerge/v2` import is dropped (the `Merger` type it referenced now lives in this same package — see `merge.go`). `JSONMerge` itself is unchanged. |
| `merge.go`             | MIT-licensed. Adapted from `apapsch/go-jsonmerge/v2` `merge.go`: verbatim except for the package declaration and the removal of the unused `MergeBytesIndent` method. |
| `jsonmerge_test.go`    | Copied verbatim from upstream `jsonmerge_test.go`.                                       |
| `merge_test.go`        | Copied from `apapsch/go-jsonmerge/v2` `merge_test.go`, minus `TestMergeBytesIndent` (which covered the removed method).                                             |
| `styleparam.go`        | **Derived**, not verbatim — see "Subset" below.                                          |
| `styleparam_test.go`   | Adapted from upstream `styleparam_test.go`: cases covering unsupported paths are dropped, and `TestStyleParamUnsupported` is added to pin down the subset boundary. |

## Subset implemented in `styleparam.go`

`StyleParamWithOptions` here supports only the inputs the access-graph client
actually emits today:

| Axis    | Supported                                                           | Not supported (returns an error)                                |
| ------- | ------------------------------------------------------------------- | --------------------------------------------------------------- |
| Styles  | `simple`, `label`, `matrix`, `form`                                 | `deepObject`, `spaceDelimited`, `pipeDelimited`                 |
| Kinds   | Primitive scalars (and their named type aliases), `time.Time`, `uuid.UUID` | Slices, maps, generic structs, `[]byte` with `format: "byte"`   |
| Formats | —                                                                   | `types.Date` (`github.com/oapi-codegen/runtime/types.Date`) is gone along with the `types` dependency |

The `StyleParam` / `StyleParamWithLocation` entry points, `StyleParamOptions`
struct, and the `ParamLocation*` constants keep the same signatures as
upstream so that generated call sites are portable.

## Guardrails

The unsupported paths above all return an error from `StyleParamWithOptions`,
and `TestStyleParamUnsupported` asserts that they do. If the access-graph
client is regenerated and starts emitting one of those shapes, the failing
test is the signal to port the matching logic from upstream rather than
silently producing malformed URLs.

## Updating

1. Diff `jsonmerge.go` and the parts of `styleparam.go` we kept against the
   corresponding files in the new upstream release, and re-apply changes.
2. If the new generated code references runtime symbols we don't vendor yet
   (for example `MarshalDeepObject`, `isByteSlice`, `types.Date`), expand the
   subset from upstream and add the matching positive tests alongside the
   existing error assertions.
3. Update the version references in this README and in each file's banner.
