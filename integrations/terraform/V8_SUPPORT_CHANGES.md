# Terraform Provider - Role v8 Support

## Summary

Added support for role version v8 in the Terraform provider. This allows users to create and manage Teleport roles with v8-specific features, particularly the `api_group` field for Kubernetes resources.

## Changes Made

### 1. Configuration Files

**File:** `protoc-gen-terraform-teleport.yaml`
- **Line 451:** Changed version validator from `UseVersionBetween(3,7)` to `UseVersionBetween(3,8)`

### 2. Generated Schema

**File:** `tfschema/types_terraform.go`
- **Line 3908:** Updated description to include `v8` in supported values
- **Line 3911:** Updated validator to `UseVersionBetween(3, 8)`

### 3. Documentation

**File:** `reference.mdx`
- **Line 114:** Added "v15+ default role version is `v8`" to version history
- **Line 1653:** Updated supported values list to include `v8`

### 4. Examples

**File:** `examples/resources/teleport_role/resource.tf`
- Updated default example to use `version = "v8"`

### 5. Test Fixtures

Created new test fixtures:
- **`testlib/fixtures/role_upgrade_v8.tf`** - Simple v8 role for upgrade testing
- **`testlib/fixtures/role_v8_full.tf`** - Comprehensive v8 role example with `api_group`

## v8 Role Features

The main difference in v8 roles for Kubernetes resources:

```hcl
# v7 role (no api_group)
kubernetes_resources = [{
  kind      = "pod"
  name      = "*"
  namespace = "*"
  verbs     = ["*"]
}]

# v8 role (requires api_group, plural kinds)
kubernetes_resources = [{
  kind      = "pods"        # Must be plural
  api_group = ""            # Required (empty string for core resources)
  name      = "*"
  namespace = "*"
  verbs     = ["*"]
}, {
  kind      = "deployments"
  api_group = "apps"        # Non-core resources need API group
  name      = "*"
  namespace = "production"
  verbs     = ["get", "list"]
}]
```

## Testing

Build verified with:
```bash
cd integrations/terraform
go build -tags "kustomize_disable_go_plugin_support,terraformprovider" -o /tmp/terraform-provider-test ./provider
```

## Next Steps

To fully regenerate schema from protobuf (requires `protoc-gen-terraform v3.0.2`):
```bash
cd integrations/terraform
make gen-tfschema
```

For documentation generation:
```bash
cd integrations/terraform
make docs
```

