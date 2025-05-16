## Teleport Terraform Provider documentation


### What documentation is available

We maintain:
- [The Terraform Provider reference](https://goteleport.com/docs/reference/terraform-provider/) describing every resource and supported field.
- [The installation guide](https://goteleport.com/docs/management/dynamic-resources/terraform-provider/)
- A getting started guide describing [how to configure users and roles with Terraform](https://goteleport.com/docs/management/dynamic-resources/user-and-role/)

### Contributing to the documentation

#### Building the reference

The full build process involves:
- a Makefile target `docs` that re-builds and installs the provider locally
- a bash script cretaing a temporary directory, exporting the schema from the built provider, invoking `tfplugindocs`
  and copying the reference into the mains docs directory.
- [a custom version of `tfplugindocs`](https://github.com/gravitational/terraform-plugin-docs) which renders markdown
  compatible with our documentation engine.

To re-render the reference, run `make docs`.

#### Adding a new resource

When adding a new resource, the file is automatically generated and included in the indices by `make docs`.

The only thing you must do is add an entry in `docs/config.json`.

#### Extending a resource reference

By default, every resource is documented with [the default template](./templates/resources.md.tmpl).
This template tries to include the example file named `examples/resources/teleport_<resource-name>/resource.tf`.

If you want to add custom text describing the resource, how it's used, or multiple examples, you can override the
default template for this resource. To do so, copy the default template to create a resource specific template:

```bash
mkdir -p templates/resources
cp templates/resources.md.tmpl templates/resources/<resource_name>.md.tmpl
```

You can then edit the resource-specific template and add custom text, include more examples, ...

To include code examples, you can use the function `tffile`:

```gotemplate
## Example Usage

<Tabs>
<TabItem label="secret token">
This is a secret provision token.

{{tffile "./examples/resources/teleport_provision_token/resource.tf" }}
</TabItem>
<TabItem label="iam token">
This is an IAM token:

{{tffile "./examples/resources/teleport_provision_token/iam.tf" }}
</TabItem>
</Tabs>
```