# Missing EKS cluster permissions

Teleport could not inspect the following EKS clusters because the integration's IAM role lacks the `eks:DescribeCluster` permission.

Add `eks:DescribeCluster` to the IAM role used by the integration. You can grant it for all EKS clusters or restrict it to the affected cluster ARNs.

Teleport retries discovery automatically. This task will expire after discovery succeeds.
