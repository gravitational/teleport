# Apply the Terraform

## Choose how to apply

Applying creates real cloud IAM resources and Teleport resources. Two questions decide how.
When Setup runs, they join Setup's question round. For a standalone apply, ask them once
with the AskUserQuestion tool before any terraform command:

- Run environment, from the table below. Default `Local workstation`.
- Who applies, with the options "Run it for me in this session" and "Give me the commands to run myself".

Do not run `terraform init`, `plan`, or `apply` until both are answered. Then follow **Local apply**
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

The process, from `write_location`:

```bash
cd <write location>
eval "$(tctl terraform env)"
terraform init
terraform plan
terraform apply
```

`tctl terraform env` exports credentials for a temporary bot that lasts one hour.

If the user applies, present the block above and stop. If you apply, run the terraform
commands directly and skip the eval, because credentials may already be in the environment.
Review the plan, and stop to get confirmation when it lists any `destroy` or `replace`
action or a matcher change. Run apply with `-auto-approve` after the plan is approved.
Never run `tctl terraform env` on its own or print its output, which contains the identity
secret.

## Confirm the apply

From the apply output, read `integration_name` and `discovery_config_name`. Link the user to the
integration's overview page in the web UI, using the host of `proxy_addr` without the port:
`https://<proxy_host>/web/integrations/overview/<integration_type>/<integration_name>`, where
`<integration_type>` is `aws-oidc` for AWS and `azure-oidc` for Azure.

## Troubleshoot the apply

| Symptom | Cause | Action |
|---------|-------|--------|
| `terraform init` reports terraform not found | Terraform is not installed | Install Terraform, or choose a non-local run environment above. |
| Provider error: credentials expired or not found | The environment has no credentials, or the `tctl terraform env` bot lapsed after one hour | Re-run the failed command in one invocation that starts with `eval "$($TCTL terraform env)"`. Exported variables do not outlive a single invocation, so keep the eval and the terraform command together. |
| Provider error: failed to connect or join (CI, cloud, server) | The bot, token, or provider auth fields are missing or wrong | Follow the environment's setup guide above. Confirm `join_method`/`join_token` or `identity_file_path` match the token created in the cluster. |
| Provider version incompatible with the cluster | The teleport provider is below the discovery module's minimum | Set the teleport provider version per the setup reference's `<provider_version>` rule. |
| `terraform apply` returns 409 on the AWS OIDC provider | An AWS OIDC provider for the proxy URL already exists | Run **Existing OIDC provider** in `references/aws-setup.md` to add the `discover.teleport` audience if missing, then re-apply with `create_aws_iam_openid_connect_provider = false`. |
| Matcher validation error | A matcher is missing a required field | Each matcher needs non-empty `types`. Azure matchers also need `subscriptions`. |
| State lock or stale state | A prior run did not release the lock | Resolve per Terraform's state-lock guidance before re-running. Do not delete state. |

## On failure

Surface the verbatim error and the write location. Do not destroy, re-apply, or retry
without approval.
