# Apply the Terraform

## Choose how to apply

Applying creates real cloud IAM resources and Teleport resources. Before running any terraform
command, ask the user once with the AskUserQuestion tool:

- Run environment, from the table below. Default `Local workstation`.
- Whether you run the apply in this session, or output the commands for the user to run themselves.

Do not run `terraform init`, `plan`, or `apply` until the user answers. Then follow **Local apply**
for a local run, or the chosen environment's setup guide otherwise.

## Run environment

Default to a local run unless the user states otherwise.

| Environment | Provider authentication | Setup guide |
|-------------|-------------------------|-------------|
| Local workstation (default) | `eval "$($TCTL terraform env)"` creates a temporary bot and exports its credentials into the shell. The provider needs only `addr`. | https://goteleport.com/docs/configuration/terraform-provider/local/ |
| CI/CD or a cloud VM | A `terraform` bot plus a delegated-join token. Set `join_method` and `join_token` on the provider. | https://goteleport.com/docs/configuration/terraform-provider/ci-or-cloud/ |
| HCP Terraform or Terraform Enterprise | A `terraform` bot plus a `terraform_cloud` token. Set `join_method = "terraform_cloud"`, `join_token`, and `audience_tag`. | https://goteleport.com/docs/configuration/terraform-provider/terraform-cloud/ |
| Dedicated server | A `tbot` daemon writes an identity file. Set `identity_file_path` on the provider. | https://goteleport.com/docs/configuration/terraform-provider/dedicated-server/ |

For a non-local environment, follow its setup guide to create the bot and token and to set
the provider auth fields, then run the same `terraform init` and `terraform apply` from that
environment.

## Local apply

The provider authenticates from credentials in the environment. If a terraform
command fails due to missing or expired credentials, ask the user to run
`eval "$(tctl terraform env)"`.

If the user will apply themselves, present these commands and stop:

> ```bash
> cd <write location>
> terraform init
> terraform apply
> ```

If the user asks you to apply:

1. Initialize:

   ```bash
   cd <write location>
   terraform init
   ```

2. Plan:

   ```bash
   cd <write location>
   terraform plan
   ```

3. Review the plan. If it lists any `destroy` or `replace` action, or a matcher change, stop,
   show what it affects, and get confirmation before applying.

4. Apply. Add `-auto-approve` only after the plan in step 3 is approved:

   ```bash
   cd <write location>
   terraform apply
   ```

## Confirm the apply

From the apply output, read `integration_name` and `discovery_config_name`. Link the user to the
integration's status page in the web UI, using the host of `proxy_addr` without the port:
`https://<proxy_host>/web/integrations/status/<integration_type>/<integration_name>`, where
`<integration_type>` is `aws-oidc` for AWS and `azure-oidc` for Azure.

## Troubleshoot the apply

| Symptom | Cause | Action |
|---------|-------|--------|
| `terraform init` reports terraform not found | Terraform is not installed | Install Terraform, or choose a non-local run environment above. |
| Provider error: credentials expired or not found | The `tctl terraform env` credentials lapsed after one hour | Ask the user to re-run `! eval "$(tctl terraform env)"` in the session, then retry the terraform command. |
| Provider error: failed to connect or join (CI, cloud, server) | The bot, token, or provider auth fields are missing or wrong | Follow the environment's setup guide above. Confirm `join_method`/`join_token` or `identity_file_path` match the token created in the cluster. |
| Provider version incompatible with the cluster | The teleport provider is below the discovery module's minimum | Set the teleport provider to `>= 18.8.0` for AWS or `>= 18.7.6` for Azure. |
| `terraform apply` returns 409 on the AWS OIDC provider | An AWS OIDC provider for the proxy URL already exists | Run **Existing OIDC provider** in `references/aws-setup.md` to add the `discover.teleport` audience if missing, then re-apply with `create_aws_iam_openid_connect_provider = false`. |
| Matcher validation error | A matcher is missing a required field | Each matcher needs non-empty `types`. Azure matchers also need `subscriptions`. |
| State lock or stale state | A prior run did not release the lock | Resolve per Terraform's state-lock guidance before re-running. Do not delete state. |

## On failure

Surface the verbatim error and the write location. Do not destroy, re-apply, or retry
without approval.
