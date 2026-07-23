# Missing EKS region permissions

Teleport could not list EKS clusters in this AWS region because the integration's IAM role lacks the `eks:ListClusters` permission.

Add `eks:ListClusters` to the IAM role used by the integration. This permission must apply to all resources because the AWS API does not support resource-level permissions for this operation.

Teleport retries discovery automatically. This task will expire after discovery succeeds.
