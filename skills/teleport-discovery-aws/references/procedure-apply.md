# Procedure: Apply the Terraform

Runs the Terraform written by `references/procedure-terraform.md`. The default is
a local run from the user's workstation. Other environments change only how the
teleport provider authenticates; this procedure points to the matching setup
guide for each.

## Run environment

Default to a local run unless the user states otherwise. The environment sets how
the teleport provider authenticates and what credentials it needs.

| Environment | Provider authentication | Setup guide |
|-------------|-------------------------|-------------|
| Local workstation (default) | `eval "$(tctl terraform env)"` creates a temporary bot and exports its credentials into the shell. The provider needs only `addr`. | https://goteleport.com/docs/zero-trust-access/infrastructure-as-code/terraform-provider/local/ |
| CI/CD (GitHub Actions, GitLab CI, CircleCI) or a cloud VM (AWS, GCP) | A `terraform` bot plus a delegated-join token. Set `join_method` and `join_token` on the provider. | https://goteleport.com/docs/zero-trust-access/infrastructure-as-code/terraform-provider/ci-or-cloud/ |
| HCP Terraform or Terraform Enterprise | A `terraform` bot plus a `terraform_cloud` token. Set `join_method = "terraform_cloud"`, `join_token`, and `audience_tag`. | https://goteleport.com/docs/zero-trust-access/infrastructure-as-code/terraform-provider/terraform-cloud/ |
| Dedicated server | A `tbot` daemon writes an identity file. Set `identity_file_path` on the provider. | https://goteleport.com/docs/zero-trust-access/infrastructure-as-code/terraform-provider/dedicated-server/ |

For a non-local environment, follow its setup guide to create the bot and token
and to set the provider auth fields, then run the same `terraform init` and
`terraform apply` from that environment.

## Local apply

1. Load temporary provider credentials into the shell. Outcome: credentials valid
   for one hour.

   ```bash
   eval "$(tctl terraform env)"
   ```

   This creates a temporary bot and exports its credentials as environment
   variables, and may prompt for MFA. Only the shell that ran it holds the
   credentials; a new shell needs the command again.

2. Initialize and apply from the write location. Outcome: resources created,
   outputs printed.

   ```bash
   cd <write location>
   terraform init -input=false
   terraform apply -input=false
   ```

   Add `-auto-approve` to the apply only when the agent runs it after the Step 3
   plan approval. When the user will run the apply themselves, leave it off so
   Terraform shows its own plan for them to confirm.

## Confirm the apply succeeded

- `terraform apply` exits 0 and prints the four outputs.
- Reaching resource creation means the provider authenticated.
- Read the created resource names from the outputs, then verify the resources and
  monitor enrollment per `references/monitor-troubleshoot.md`.

## Troubleshoot the apply

| Symptom | Cause | Action |
|---------|-------|--------|
| Provider error: credentials expired or not found (local) | The `tctl terraform env` credentials lapsed after one hour, or the apply ran in a different shell | Re-run `eval "$(tctl terraform env)"` in the shell you apply from. |
| Provider error: failed to connect or join (CI, cloud, server) | The bot, token, or provider auth fields are missing or wrong | Follow the environment's setup guide above. Confirm `join_method`/`join_token` or `identity_file_path` match the token created in the cluster. |
| Provider version incompatible with the cluster | The pinned teleport provider major version does not match the cluster | Pin the teleport provider to the cluster's major version from `cluster_version`. The module requires the provider `>= 18.8.0`. |
| `terraform apply` 409 on the OIDC provider | An AWS OIDC provider for the proxy URL already exists | Set `create_aws_iam_openid_connect_provider = false` and re-apply. |
| `terraform apply` fails on `aws_matchers` validation | `aws_matchers` is empty, or a matcher is missing `types` or `regions` | Fix the matcher. Each needs non-empty `types` and `regions`. |
| State lock or stale state | A prior run did not release the lock | Resolve per Terraform's state-lock guidance before re-running. Do not delete state. |

## On failure

Surface the verbatim error and the write location. Do not destroy or re-apply
without approval.
