# Teleport resource converter

The Teleport resource converter is a small utility for transforming `tctl`
resource documents into Terraform resource configurations and Kubernetes
resource manifests. 

The converter is intended for hand-authored resource documents, and not those
returned directly from the Teleport Auth Service backend, since it does not
contain logic for handling fields that track a resource's internal state.

## How to build

You will need Go installed on your system. In this directory, run the following
command:

```sh
$ go build .
```

## Basic usage

This is a CLI that reads a `tctl` resource YAML document from stdin (potentially
including multiple resources, divided by YAML document separators) and returns
the converted HCL configuration or Kubernetes YAML to stdout.

The `-format` flag indicates which infrastructure as code tool format to use.
Valid values are:

- `hcl`
- `kube`

For example, you could convert a role to HCL using a heredoc:

```sh
cat<<EOF | ./convert-resource -format=hcl
kind: role
version: v8
metadata:
  name: example
spec:
  allow:
    logins: [ubuntu]
EOF
```

## Exit codes

The program uses exit codes to help invoking shells determine whether a given
`tctl` resource has no corresponding Terraform or Kubernetes resource, or
whether the conversion failed for some other reason (e.g., malformed YAML):

| Code | Purpose                                           |
|------|---------------------------------------------------|
| 2    | The resource is not supported in the given format |
| 1    | All other errors                                  |
| 0    | The conversion took place successfully

## Supported kinds

The `resourceConfig` map configures the resources supported for conversion.
