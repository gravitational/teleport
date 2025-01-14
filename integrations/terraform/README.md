# Terraform Provider Plugin

## Usage

Please, refer to [official documentation](https://goteleport.com/docs/admin-guides/infrastructure-as-code/terraform-provider/terraform-provider/).

## Development

1. Install [`protobuf`](https://grpc.io/docs/protoc-installation/).
2. Install [`protoc-gen-terraform`](https://github.com/gravitational/protoc-gen-terraform) @v3.0.2.

    ```go install github.com/gravitational/protoc-gen-terraform/v3@v3.0.2```

3. Install [`Terraform`](https://learn.hashicorp.com/tutorials/terraform/install-cli) v1.1.0+. Alternatively, you can use [`tfenv`](https://github.com/tfutils/tfenv). Please note that on Mac M1 you need to specify `TFENV_ARCH` (ex: `TFENV_ARCH=arm64 tfenv install 1.1.6`).

4. Build and install the plugin:

    ```bash
    cd integrations/terraform
    make install
    ```

5. Run tests:

    ```bash
    make test
    ```

# Updating the provider

Run:

```
make gen-tfschema
```

This will generate `types_tfschema.go` from a current API `.proto` file, and regenerate the provider code.

If the schema changed, you will need to re-render the docs with:
```
make docs
```

# Playing with examples locally

1. Start Teleport.

    ```
    teleport start
    ```

1. Create Terraform user and role:

    ```
    tctl create example/terraform.yaml
    tctl auth sign --format=file --user=terraform --out=/tmp/terraform-identity --ttl=10h
    ```

1. Create `main.tf` file:

    ```
    cp example/main.tf.example example/main.tf
    ```

    Please note that target identity file was exported to `/tmp/terraform-identity` on previous step. If you used another location, please change in in `main.tf`.

1. Create sample resources:

    ```
    cp example/user.tf.example example/user.tf
    cp example/role.tf.example example/role.tf
    cp example/provision_token.tf.example example/provision_token.tf
    ```

    Please note that some resources require preliminary setup steps.

1. Apply changes:

    ```
    make apply
    ```

1. Make changes to .tf files and run:
    ```
    make reapply
    ```

1. Clean up:
    ```
    make destroy
    ```
