---
authors: Paul Gottschling (paul.gottschling@goteleport.com)
state: draft
title: 0130 - Automatically Generate the Teleport Resource Reference
---

## Required Approvers

- Engineering: @codingllama, @zmb3
- Product: (@alexfornuto || @xinding33)

## What

Dynamic resources (also called "Teleport resources" in this RFD) are
configuration objects that Teleport users can apply using `tctl`, Terraform, and
other methods.

The scope of this RFD is to automatically generate reference documents for
resources you can apply via `tctl` from the Teleport source code. 

To do this, we can write a program that we can run as a new Make target in a
clone of `gravitational/teleport`. When we make changes to Teleport's dynamic
resources, we can run the program manually, generate a new iteration of the
reference, and manually open a pull request with the new content.

The Make target will depend on `make grpc` so the generator always works from
the latest `proto` files.

## Why

We want to ensure that the Teleport resource reference is complete and up
to date so Teleport operators can manage their clusters without creating support
load.

In general, we should strive to automatically generate reference guides whenever
possible to ensure accuracy. While adding entries incrementally over time can be
sustainable at a certain scale, adding many entries at once is a daunting task
that is difficult for a documentation team to schedule. Our `tctl` resource
reference is particularly lagging, with only five resources documented at the
[time of
writing](https://github.com/gravitational/teleport/blob/58f20f0c3fcc9bb281528915125b95dfaee6cec5/docs/pages/reference/resources.mdx).

## Details

After parsing Go source files, the generator uses AST information to [populate a
template](#the-output). To do so, it builds
[mappings](#working-with-parsed-go-source) that it can use to track type
declarations in source packages. Using these mappings, it
[converts](#converting-structs-to-resources) struct types to template data. To
document the types of struct fields, the generator writes [additional
entries](#processing-field-types) to the reference template as appropriate.

### Configuration

By default, the generator will attempt to include an entry in the resource
reference for every struct type in the Teleport source that includes a field of
the `github.com/gravitational/teleport/api/types.Metadata` type, since we
include this field in all types that correspond to Teleport resources.

For resources that we want to _exclude_ from the reference, the generator will
read from a mapping of package paths to lists of named struct types within a
configuration file. 

#### Alternative: Working from protobuf message definitions

We could write a `protoc` plugin that generates the Teleport resource reference
from our protobuf message definitions. This is how we generate our Terraform
provider source in `github.com/gravitational/protoc-gen-terraform`.

However, we can't guarantee that every Teleport resource available to `tctl`
will correspond to a protobuf message definition. Since we generate Go source
files based on our `.proto` files, and Teleport needs to unmarshal all Teleport
resources into structs from YAML, it's a safe bet that all Teleport resources
will have a corresponding Go struct.

The downside of using Go source files is that, if we want to change the
description of a field or some other aspect of the generated reference, we need
to: 

1. Locate the Go struct that corresponds to an entry within the reference page.
1. Determine if the Go struct was generated from a `proto` file and, based on
   this, edit either the Go struct definition or the underlying `proto` file.
1. If relevant, generate a Go source file from the edited protobuf message
   definition.
1. Regenerate the reference page.

I don't imagine this will be prohibitive, though, as long as we:

- Indicate within the generated reference page which Go source file each entry
  is based on.
- Edit the comments in Go type definitions and `proto` files to ensure
  consistency, clarity, and grammatical correctness.

Having this Make target depend on `make grpc` will also automate this flow.

### Working with parsed Go source

The generator will use
[`packages.Load`](https://pkg.go.dev/golang.org/x/tools/go/packages#Load) to
load all the packages in the `github.com/gravitational/teleport` source and
return a `[]*packages.Package`. It then recursively traverses each
`packages.Package`'s `Imports` map, adding to a final slice of parsed packages.

The generator declares two maps. The key for each map, `declKey`, includes the
package path and named type of a declaration:

- Declaration map: `map[declKey]sourceDecl`: Looking up AST data for a given
  declaration in order to build template data. A `sourceDecl` is a struct with
  the following definition:

  ```go
  type sourceDecl struct {
   decl  *ast.Decl // Declaration data
   file  string   // File path where the declaration was made
  }
  ```

- Final data map: `map[declKey]Resource`: Tracking type data to use to populate
  a template. This ensures that each declaration is only included once.

In each `*packages.Package` in the slice mentioned earlier, the generator ranges
through the package's `Syntax` field, which is an `[]*ast.File`. Within each
`*ast.File`, the generator ranges through the file's
[`Decls`](https://pkg.go.dev/go/ast#File) field and adds each declaration to the
declaration map. The `CompiledGoFiles` field in `packages.Package` lists paths
of Go source files in the same order as the `Syntax` field, so we can use this
to populate the `file` property of each `sourceDecl` in the map.

The next step is to look up each struct type within the declaration map and
begin generating a `Resource` for that struct type, plus additional `Resource`
structs for the types of its fields (and their types and so on). The sequence
is:

- Iterate through each struct type within the declaration map.
- If a struct type is named within the configuration as a type to exclude, skip
  it.
- If a struct type has a `types.Metadata` field, construct a `Resource` from the
  struct type and insert it into the final data map (unless a `Resource` already
  exists there).
- Process the fields of the struct type using the rules described
  [below](#processing-field-types), looking up declaration data from the
  declaration map.

Once this is done, the generator will convert the  `map[declKey]Resource` into a
single `[]Resource` to feed into the template.

#### Alternative: Using analysis drivers

The Go Tools repository provides the `multichecker`
(https://pkg.go.dev/golang.org/x/tools/go/analysis/multichecker) and
`singlechecker`
(https://pkg.go.dev/golang.org/x/tools/go/analysis/singlechecker) packages
within `golang.org/x/tools/go/analysis`, which run custom static analysis
functions, `analysis.Analyzer`s. However, these static analysis tools are
intended for linting, not generating reference documentation. 

While you can configure an `analysis.Analyzer` to return a value from its `Run`
function, the `multichecker` and `singlechecker` drivers run by exposing a
`Main` function that handles all of the program's output and CLI arguments. It
is not possible to initialize a data structure outside the `Main` function and
assign values to it during the `Run` function, since `Main`
[exits](https://cs.opensource.google/go/x/tools/+/refs/tags/v0.8.0:go/analysis/multichecker/multichecker.go;l=59)
the current process after completing, leaving the program no chance to process
the data structure.

The `singlechecker` and `multichecker` packages run static analysis by importing
from a `checker` package, which is internal to the
`analysis` package. While we could copy this code and
modify it for our purposes, the reference generator does not require the
general-purpose static analysis capabilities of the `analysis` package.

### Converting structs to `Resource`s

In Go's `ast` package, a [`Decl`](https://pkg.go.dev/go/ast#Decl) is an
interface representing a declaration. The logic for generating a `Resource`
from a given type declaration depends on the nature of the type declaration. 

First, check whether a given `struct` declaration has a custom `UnmarshalJSON`
or `UnmarshalYAML` method. In this case, treat the type as a [custom
type](#custom-fields) and print a message.

Next, if there is no custom unmarshaler for the type, assert that the `Decl` is
an `*ast.GenDecl` with a `Specs` containing a `TypeSpec` with `Type:
StructType`. From there, we can map the fields within
[`ast.StructType`](https://pkg.go.dev/go/ast#StructType) to fields in a new
`Resource`.

Each `StructType` has a [`*FieldList`](https://pkg.go.dev/go/ast#FieldList) that
we can use to obtain field data, including comments and tags. The generator
ignores fields that are not exported.

|`Resource` Field|How to generate|
|---|---|
|`Resource.SourcePath`|The `file` property of the underlying `sourceDecl`.|
|`Resource.Description`|`GenDecl.Doc`|
|`Resource.SectionName`|`TypeSpec.Name.Name`, with camelcase converted to separate words.|
|`Fields[*].Name`|Process the `Field.Tag` in the `FieldList`, extracting the value of the `json` tag. If there is no `json` tag, use the original struct field name.|
|`Fields[*].Description`|Each [`Field`](https://pkg.go.dev/go/ast#Field) in a `FieldList` has a `*CommentGroup` that we can use to extract the field's GoDoc. Replace mentions of the field name in the GoDoc with the value of `Fields[*].Name`. If there is no GoDoc for the field, exit with a descriptive error so we can manually add one.|
|`Fields[*].Type`|We can use `Field.Type` within the `FieldList` for this. Follow the rules for processing field types [below](#processing-field-types) |
|`YAMLExample`|Follow the rules for processing field types [below](#processing-field-types).|

If a root type named within the config file is not a struct, the generator will
exit with an error message.

### Processing field types

#### Custom fields

If a field of a struct type has a custom type, we will include a link to an
entry for that type in the struct type's table. The YAML example will include an
ellipsis, leaving it to the custom field's entry in the reference to provide the
example:

```markdown
## My Resource

|Field Name|Description|Type|
|---|---|---|
|`name`|The resource's name.|`string`|
|`labels`|The resource's labels.|[Labels](#labels)|

\`\`\`yaml
`name`: "string"
`labels`:
# ...
\`\`\`
```

Entries in the reference for custom fields, e.g., `Labels`, don't lend
themselves to automatic generation because they implement custom unmarshalers,
and we can't rely on `json` struct tags to indicate the types of these fields.
For the following field types, leave the `Type` column of the field table blank,
and do not attempt to populate an example YAML value:

- Scalars with a custom type
- Custom types declared outside the source tree, or in the standard library
- Interfaces
- Types with a custom unmarshaler

#### Predeclared scalar types

If a struct field has a predeclared scalar type (e.g., `int` or `string`), we
don't need a new entry in the reference. Instead, the generator supplies a type
for the field table by converting the field's Go type to the appropriate YAML
type, e.g., `number` for `int64`.

In the YAML example, we use the field's JSON struct tag to extract a field
name.  We use predefined examples based on the primitive type to provide the
field's value, e.g., `"string"` for a string and `1` for an integer.

If a manual override file exists for a field, the generator uses that instead
when populating the YAML example. In this case, the generator populates examples
of that field within YAML snippets with ellipses as described in the previous
section. This way, we won't have to deal with unintended formatting effects of
inserting hardcoded YAML examples into automatically generated ones.

#### Predeclared composite types

If a field has a predeclared composite type, e.g., `map[string]string` or
`[]int`, include the field in the table. If one element of a predeclared
composite type is a named type, include a link to a section for that named type
within the reference. An example of this:

```markdown
|Name|Description|Type|
|---|---|---|
|`dynamic_labels`|The app's command labels.|`map[string]`[Command Label V2](#command-label-v2)|
```

In the example YAML block for a given type, print example values depending on
the composite type.

For maps, include three sample keys and values based on the logic for printing
predeclared scalar types. E.g., for a field called `mymap` of type
`map[string]int`:

```yaml
mymap:
  key1: 1
  key2: 1
  key3: 1
```

For slices, the logic is similar. For a field called `myslice` of type `[]int`,
we would use:

```yaml
myslice:
 - 1
 - 1
 - 1
```

...and for `[]string`:

```yaml
myslice:
  - "string"
  - "string"
  - "string"

Recursively handle composite types. For example, `map[string][]string` would
result in:

```yaml
mymap:
  key1:
   - "string"
   - "string"
   - "string"
  key2:
   - "string"
   - "string"
   - "string"
  key3:
   - "string"
   - "string"
   - "string"
```

If the composite type includes a named type, e.g., a field called `headers` of
type `[]Header` or `map[string]Header`, print an ellipsis to avoid undue
complexity:

```yaml
headers:
# ...
```

In a nested composite type, we would only print the ellipsis for the innermost
composite type. For example, a field called `field` of type
`map[string][]Header` would appear like this:

```yaml
field:
  key1:
  # ...
  key2:
  # ...
  key3:
  # ...
```

The same rule would apply to, for example, `map[string]map[string]Header`.

## Test plan

We can make it part of our release procedure to ensure that we run the generator
when we release a new version of Teleport.
