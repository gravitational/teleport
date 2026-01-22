# Missing permission: account:ListRegions
Teleport failed to discover EC2 instances because the integration's IAM role lacks permission to list AWS regions.

This error occurs when using `regions: ["*"]` (wildcard) in the EC2 matcher configuration, which requires Teleport to call `account:ListRegions` to determine which regions to scan.

**How to fix**

Add the `account:ListRegions` permission to the IAM role used by the integration:

```json
{
  "Effect": "Allow",
  "Action": "account:ListRegions",
  "Resource": "*"
}
```

**Alternative fix**

Instead of using `regions: ["*"]`, specify explicit region names in your matcher configuration:

```yaml
regions:
  - us-east-1
  - us-west-2
  - eu-west-1
```

After applying the fix, mark this task as resolved. Teleport will retry discovery automatically.
