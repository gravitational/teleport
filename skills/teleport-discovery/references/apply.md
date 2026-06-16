# Apply the Terraform

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

`eval "$($TCTL terraform env)"` exports short-lived credentials and may prompt for MFA. Its
variables do not persist between Bash calls, so chain it with each `terraform plan` or
`terraform apply` command.

If the user will apply themselves, present these commands and stop. Do not show the
`eval ... &&` chaining in commands you present to the user:

> ```bash
> cd <write location>
> terraform init
> eval "$(tctl terraform env)"
> terraform apply
> ```

If the user asks you to apply:

1. Initialize and plan:

   ```bash
   cd <write location>
   terraform init
   eval "$($TCTL terraform env)" && terraform plan
   ```

2. Review the plan. If it lists any `destroy` or `replace` action, or a matcher change, stop,
   show what it affects, and get confirmation before applying.

3. Apply. Add `-auto-approve` only after the plan in step 2 is approved, or when the caller
   passed `auto_approve: true`:

   ```bash
   cd <write location>
   eval "$($TCTL terraform env)" && terraform apply -auto-approve
   ```

## Confirm the apply

From the apply output, read `integration_name` and `discovery_config_name`. Link the user to the integration in the web UI,
using the host of `proxy_addr` without the port: `https://<proxy_host>/web/integrations`.

## Troubleshoot the apply

| Symptom | Cause | Action |
|---------|-------|--------|
| `terraform init` reports terraform not found | Terraform is not installed | Install Terraform, or choose a non-local run environment above. |
| Provider error: credentials expired or not found | The `tctl terraform env` credentials lapsed after one hour, or the apply ran in a different shell | Re-run `eval "$($TCTL terraform env)"` in the shell you apply from. |
| Provider error: failed to connect or join (CI, cloud, server) | The bot, token, or provider auth fields are missing or wrong | Follow the environment's setup guide above. Confirm `join_method`/`join_token` or `identity_file_path` match the token created in the cluster. |
| Provider version incompatible with the cluster | The teleport provider is below the discovery module's minimum | Set the teleport provider to `>= 18.8.0` for AWS or `>= 18.7.6` for Azure. |
| `terraform apply` returns 409 on the AWS OIDC provider | An AWS OIDC provider for the proxy URL already exists | Set `create_aws_iam_openid_connect_provider = false` and re-apply. |
| Matcher validation error | A matcher is missing a required field | Each matcher needs non-empty `types`. Azure matchers also need `subscriptions`. |
| State lock or stale state | A prior run did not release the lock | Resolve per Terraform's state-lock guidance before re-running. Do not delete state. |

## On failure

Surface the verbatim error and the write location. Do not destroy, re-apply, or retry
without approval.
