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

- `destination` (string): the directory path in which to place reference pages.

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
