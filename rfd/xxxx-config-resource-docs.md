---
authors: Paul Gottschling (paul.gottschling@goteleport.com)
state: draft
title: NNN - Automatically Generate the Configuration Resource Reference
---

## Required Approvers

- Engineering: @codingllama
- Product: (@alexfornuto || @xinding33)

## What

Automatically generate the "Dynamic resources" section of the configuration
resource reference (`docs/pages/reference/resources.mdx`), a list of the
resources you can apply via `tctl`, from the Teleport source code. 

To do this, write a program that we can run as a new Make target in a clone of
`gravitational/teleport`. When we make changes to Teleport's dynamic
configuration resources, we can run the program manually, generate a new
iteration of the reference, and manually open a pull request with the new
content.

## Why

We want to ensure that the configuration resource reference is complete and up
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

The generator reads a [configuration](#configuration) that specifies struct
types to generate references from. After parsing Go source files, the generator
uses the resulting AST information to [populate a template](#the-output). 

To do so, it builds [mappings](#working-with-parsed-go-source) that it can use
to track type declarations in source packages. Using these mappings, it
[converts](#converting-structs-to-resources) struct types to template data. To
document the types of struct fields, the generator writes [additional
entries](#processing-field-types) to the reference template as appropriate.

### Configuration

The generator will have access to a mapping of package paths to lists of named
struct types within each package. It will use this mapping to identify the
struct types to use for generating the resource reference. 

Since struct definitions for `tctl` resources include field types that also
require their own entries in the resource reference (e.g., `types.Metadata`),
the configuration only names the _root_ types for the generator to examine.

We don't have a formal naming convention for the root types of each resource,
and may not want to list *all* resources (e.g., there may be internal resources
to keep hidden), so it would make sense to list resources explicitly. The CRD
generator for the Teleport Kubernetes Operator [uses an explicit list of
types](https://github.com/gravitational/teleport/blob/c9b0a601ab781e1bd34dc3c44a94e6e85e3e64ae/integrations/operator/crdgen/main.go#L96-L113),
and the generator I'm proposing will also include package names so the generator
loads the correct source packages and their dependencies.

The program could read the mapping from a YAML configuration file or declare it
directly as a Go `map`.

Here is an example in YAML:

```yaml
github.com/gravitational/teleport/api/types
- ProvisionTokenV2
- AuthPreferenceV2
- AccessRequestV3

github.com/gravitational/teleport/api/loginrulev1
- LoginRule
```

#### Alternative: Working from protobuf message definitions

We could write a `protoc` plugin that generates the configuration resource
reference from our protobuf message definitions. This is how we generate our
Terraform provider source in `github.com/gravitational/protoc-gen-terraform`.

However, we can't guarantee that every configuration resource available to
`tctl` will correspond to a protobuf message definition. Since we generate Go
source files based on our `.proto` files, and Teleport needs to unmarshal all
configuration resources into structs from YAML, it's a safe bet that all
configuration resources will have a corresponding Go struct.

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

### The output

The output of the program will be a configuration resource reference generated
from a template.

#### Template format and data

The generator will produce a template based on a `Resource` struct with the
following fields:

```go
type Resource struct {
  SectionName string
  Description string
  SourcePath  string
  Fields      []Field
  YAMLExample string
}

type Field struct{
  Name        string
  Description string
  Type        string
}
```

The template will look similar to this, anticipating a `[]Resource`:

```text
{{ range . }}
## {{ .SectionName }}

{{ .Description }}

{/*Automatically generated from: {{ .SourcePath}}*/}

|Field Name|Description|Type|
|---|---|---|
{{ range .Fields }}
|.Name|.Description|.Type|
{{ end }} 

{{ .YAMLExample }}
{{ end }}
```

#### Template example

Here is an example of a populated template for an application (`AppSpecV3` in
`api/types/types.pb.go`): 

```markdown
## App Spec V3

App Spec V3 is the specification for an application registered with Teleport.

{/*Automatically generated from: api/types/types.pb.go*/}

|Field Name|Description|Type|
|---|---|---|
|`uri`|The web app endpoint.|`string`|
|`public_addr`|The public address the application is accessible at.|`string`|
|`dynamic_labels`|The app's command labels.|`map[string]`[Command LabelV2](#command-label-v2)|
|`insecure_skip_verify`|Disables app's TLS certificate verification.|`boolean`|
|`rewrite`|A list of rewriting rules to apply to requests and responses.|[Rewrite](#rewrite)|
|`aws`|Contains additional options for AWS applications.|[App AWS](#app-aws)|
|`cloud`|Identifies the cloud instance the app represents.|`string`|

\`\`\`yaml
uri: string
public_addr: string
dynamic_labels:
 # ...
insecure_skip_verify: true
rewrite: 
 # ...
aws:
 # ...
cloud:
 # ...
\`\`\`

## Command Label V2

Command Label V2 is a label that has a value as a result of the output generated
by running command, e.g. hostname.

{/*Automatically generated from: api/types/types.pb.go*/}

|Field Name|Description|Type|
|---|---|---|
|`period`|A time between command runs.|[Duration](#duration)|
|`command`|A command to run|`[]string`|
|`result`|Captures standard output|`string`|

\`\`\`yaml
period: 10s
command:
- string
- string
- string
result: string
\`\`\`

## Duration

Duration is a Go [duration type](https://pkg.go.dev/time#Duration).

{/*Loaded from example file at autogen/examples/duration.mdx*/}

\`\`\`yaml
10s
\`\`\`

## Rewrite

Rewrite is a list of rewriting rules to apply to requests and responses.

{/*Automatically generated from: api/types/types.pb.go*/}

|Field Name|Description|Type|
|---|---|---|
|`redirect`|Defines a list of hosts which will be rewritten to the public address of the application if they occur in the "Location" header.|`[]string`|
|`headers`|A list of headers to inject when passing the request over to the application.|`[]`[Header](#header)|

\`\`\`yaml
redirect: 
- string
- string
- string
headers:
# ...
\`\`\`

## Header

Header represents a single http header passed over to the proxied application.

{/*Automatically generated from: api/types/types.pb.go*/}

|Field Name|Description|Type|
|---|---|---|
|`name`|The http header name.|`string`|
|`value`|The http header value.|`string`|

\`\`\`yaml
name: string
value: string
\`\`\`
```

### Working with parsed Go source

The generator will use
[`packages.Load`](https://pkg.go.dev/golang.org/x/tools/go/packages#Load) to
load all the packages named in generator's configuration file and return a
`[]*packages.Package`. It then recursively traverses each `packages.Package`'s
`Imports` map, adding to a final slice of parsed packages.

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

The next step is to look up each user-configured struct type within the
declaration map and begin generating a `Resource` for that struct type, plus
additional `Resource` structs for the types of its fields (and their types
and so on). The sequence is:

- Look up a user-defined struct type from the configuration within the
  declaration map.
- Construct a `Resource` from the struct type and insert it into the final data
  map (unless a `Resource` already exists there).
- Process the fields of the user-defined struct type using the rules described
  [below](#processing-field-types).

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

Start by asserting that the `Decl` is an `*ast.GenDecl` with a `Specs`
containing a `TypeSpec` with `Type: StructType`. From there, we can map the
fields within [`ast.StructType`](https://pkg.go.dev/go/ast#StructType) to fields
in a new `Resource`.

Each `StructType` has a [`*FieldList`](https://pkg.go.dev/go/ast#FieldList) that
we can use to obtain field data, including comments and tags. The generator
ignores fields that are not exported.

|`Resource` Field|How to generate|
|---|---|
|`Resource.SourcePath`|The `file` property of the underlying `sourceDecl`.|
|`Resource.Description`|`GenDecl.Doc`|
|`Resource.SectionName`|`TypeSpec.Name.Name`, with camelcase converted to separate words.|
|`Fields[*].Name`|Process the `Field.Tag` in the `FieldList`, extracting the value of the `json` tag.|
|`Fields[*].Description`|Each [`Field`](https://pkg.go.dev/go/ast#Field) in a `FieldList` has a `*CommentGroup` that we can use to extract the field's GoDoc. If a comment begins with the name of the field, we can remove the field name (and "is" if it follows the field name) and capitalize the new first letter of the GoDoc (e.g., `MyField is a field` becomes `A field`). If there is no GoDoc for the field, exit with a descriptive error so we can manually add one.|
|`Fields[*].Type`|We can use `Field.Type` within the `FieldList` for this. Follow the rules for processing field types [below](#processing-field-types) |
|`YAMLExample`|Follow the rules for processing field types [below](#processing-field-types).|

If a root type named within the config file is not a struct, the generator will
exit with an error message.

### Processing field types

#### Custom fields

Entries in the reference for custom fields, e.g., `Labels`, don't lend
themselves to automatic generation because they implement custom unmarshalers,
and we can't rely on `json` struct tags to indicate the types of these fields.

For simplicity, we can describe custom fields by hardcoding their descriptions
and example YAML values. To do so, we can include an `examples` directory within
the project directory for the generator. We can define a convention that the
generator will use to look up the name of a field from a filename within the
directory, e.g., `labels.md` for `Labels`. Each example file contains a
description and example YAML snippet.

This approach also gives us flexibility in whether to include a table of fields.
Entries for custom field types like `Labels`, for example, would not include a
table of fields, but rather a description of how labels work.

If a field has a custom type, we will include a link to an entry for that type
in the field's table. The YAML example will include an ellipsis, leaving it to
the custom field's entry in the reference to provide the example:

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

Here is an example of an override file called `labels.md`:

```markdown
## Labels

Labels are an object where keys are strings. The value of each key can either be
a list of strings or a single string. You can use wildcards or regular
expressions.

\`\`\`yaml
'env': 'test'
'*': '*'
'region': ['us-west-1', 'eu-central-1']
'reg': '^us-west-1|eu-central-1$'
\`\`\`
```

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

#### Named types

For both named scalar types and named composite types, if the named type is not
a struct, the generator looks for a manual override and returns an error if one
is not available. This is because, for named composite types (e.g., `Labels`),
it's likely that the source contains some custom unmarshaling logic that we will
need to explain.

If the type of a field is a named struct, the generator will produce another
`Resource` based on that struct and insert it elsewhere in the reference
template.

For all named field types, the reference entry for the struct that contains the
named field type will include an ellipsis in the example YAML. For example, the
`CommandLabelV2` type appears within the `AppSpecV3` struct in the
`dynamic_labels` field as shown below:

```yaml
uri: string
public_addr: string
dynamic_labels:
 # ...
insecure_skip_verify: true
rewrite: 
 # ...
aws:
 # ...
cloud:
 # ...
```

Users can then navigate to the `Command Label V2` section (using the link in the
table above the example YAML) to see an example YAML document for this type.

If the type of the field is an embedded struct, the generator will act as if the
fields of the embedded struct are fields within the outer struct. Otherwise, the
generator will follow the rules above.

## Test plan

We can add an item to the "Documentation" section of the test plan ensuring that
we run the generator (in addition to other docs generators) when we release a
new version of Teleport.
