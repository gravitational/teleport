## Contributing

### Testing local builds 

Configure `dev_overrides` in `~/.terraformrc`, e.g:

```hcl
provider_installation {

  dev_overrides {
      "terraform.releases.teleport.dev/gravitational/teleportmwi" = "/Users/noah/code/teleport/integrations/terraform-mwi/build"
  }

  # For all other providers, install them directly from their origin provider
  # registries as normal. If you omit this, Terraform will _only_ use
  # the dev_overrides block, and so no other providers will be available.
  direct {}
}
```

and then run `make build`.

See https://developer.hashicorp.com/terraform/cli/config/config-file#development-overrides-for-provider-developers
for more information.