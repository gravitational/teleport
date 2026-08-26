# Resource reference generator

The resource reference generator is a Go program that produces a comprehensive
reference guide for all dynamic Teleport resources and the fields they include.
It uses the Teleport source as the basis for the guide. 

## Usage

From the root of your `gravitational/teleport` clone:

```
$ make gen-resource-docs
```

## How it works

The resource reference generator works by:

1. Identifying Go types that represent to the fields of each dynamic resource
   struct identified in a configuration file.
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

- `source` (string): the path to the root of a Go project directory.

- `destination` (string): the directory path in which to place reference pages.

- `resources` (array of resource configuration objects): Teleport dynamic
  resources to represent in the reference docs.

### Resource configuration

In the `resources` field of the configuration file, each resource configuration
object has the following structure:

- `type`: The name of the struct type declaration that represents the resource,
  e.g., `RoleV6`.
- `package`: The name of the Go package that includes the type declaration,
  e.g., `types`.
- `yaml_kind`: The value of `kind` to include in the resource manifest, e.g.,
  "role".
- `yaml_version`: The value of `version` to include in the resource manifest,
  e.g., "v6".

### Example

```yaml
source: "../../../../api"
destination: "../../../../docs/pages/reference/tctl-resources"
resources:
  - type: RoleV6
    package: types
    yaml_kind: "role"
    yaml_version: "v6"
  - type: OIDCConnectorV3
    package: types
    yaml_kind: "oidc"
    yaml_version: "v3"
  - type: SAMLConnectorV2
    package: types
    yaml_kind: "saml"
    yaml_version: "v2"
```
