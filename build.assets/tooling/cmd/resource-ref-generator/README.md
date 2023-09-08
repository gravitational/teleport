# Resource reference generator

The resource reference generator is a Go program that produces a comprehensive
reference guide for all dynamic Teleport resources and the fields they include.
It uses the Teleport source as the basis for the guide. 

## Usage

```
$ cd build.assets/tooling/cmd/resource-ref-generator
$ go run . -config config.yaml
```

## How it works

The resource reference generator works by:

1. Identifying Go structs that correspond to dynamic Teleport resources.
1. Identifying Go types that represent to the fields of each dynamic resource
   struct.
1. Retrieving reference information about dynamic resources and their fields
   using a combination of Go comments and type information.

### Example YAML overrides

Each entry in the resource reference includes an example YAML document. While
the reference generator attempts to construct the YAML document from Go type
information, you can instruct it to use a hardcoded YAML example by editing the
Go comment above a declaration. 

This example modifies the `proto` file that declares the `MFADevice` message in
order to override its example YAML. When we generate Go code from this message
definition and run the generator, it uses the override instead of Go type
information to add the YAML example:

```proto
// MFADevice is a multi-factor authentication device, such as a security key or
// an OTP app.
// Example YAML:
// ---
// kind: mfa_device
// version: v1
// metadata:
//  name: "string"
// id: 00000000-0000-0000-0000-000000000000
message MFADevice {
```

The generator assumes that all comments after the following string belong to the
YAML example:

```go
// Example YAML:
// ---
```

You can also provide a hardcoded YAML example above a single struct field. In
this case, the generator will generate example YAML content for all other fields
in the struct and use the hardcoded YAML for the overridden field. 

For example, this source code overrides the `LabelMaps` field while allowing the
generator to document the `Name` field automatically:

```go
// Server includes information about a server registered with Teleport.
type Server struct {
    // Name is the name of the server.
    Name string `json:"name"`
    // LabelMaps includes a map of strings to labels.
    // Example YAML:
    // ---
    //
    // - label1: ["my_value0", "my_value1", "my_value2"]
    //   label2: ["my_value0", "my_value1", "my_value2"]
    // - label3: ["my_value0", "my_value1", "my_value2"]
    LabelMaps []map[string]types.Label `json:"label_maps"`
}
```

If a YAML example is above an entire type declaration, the generator will not
produce a table of fields. If only one field is overridden, the generator *will*
produce a table of fields, and refer the reader to the example YAML for any
field that is overridden.

### Editing source files

The resource reference indicates which Go source files the reference generator
used for each entry. The generator is only aware of Go source files, not
protobuf message definitions.

If a source file is based on a protobuf message definition, edit the message
definition first, then run:

```
$ make grpc
```

After that, you can run the reference generator.

## Configuration

The generator uses a YAML configuration file with the following fields.

### Main config

- `required_field_types`: a list of type info mappings (see "Type info")
  indicating type names of fields that must be present in a dynamic resource
  before we include it in the reference. For example, if this is `Metadata` from
  package `types`, a struct type must include a field with the a field of
  `types.Metadata` before we add it to the reference.

- `source` (string): the path to the root of a Go project directory.

- `destination` (string): the file path of the resource reference.

- `excluded_resource_types`: a list of type info mappings (see "Type info")
  indicating names of resources to exclude from the reference. 

- `field_assignment_method`: the name of a method of a resource type that
  assigns fields to the resource. Used to identify the kind and version of a
  resource.

### Type info

- `package`: The path of a Go package
- `name`: The name of a type within the package

### Example

```yaml
required_field_types:
  - name: Metadata
    package: api/types
  - name: ResourceHeader
    package: api/types
source: "api"
destination: "docs/pages/includes/resource-reference.mdx"
excluded_resource_types:
  - package: "types"
    name: "ResourceHeader"
field_assignment_method: "setStaticFields"
```
