# Missing AWS organization permissions

Teleport failed to discover EC2 instances because the integration's IAM role lacks the required permissions to discover AWS accounts in your organization.

This error occurs when using organization-wide discovery, which requires Teleport to list accounts across your AWS organization.

**How to fix:**

Add the following permissions to the IAM role used by the integration:

```json
{
  "Effect": "Allow",
  "Action": [
    "organizations:ListRoots",
    "organizations:ListChildren",
    "organizations:ListAccountsForParent"
  ],
  "Resource": ["*"]
}
```

These permissions allow Teleport to:

- `organizations:ListRoots` - Find the root of your organization
- `organizations:ListChildren` - Navigate organizational units (OUs)
- `organizations:ListAccountsForParent` - List accounts within each OU

After applying the fix, mark this task as resolved. Teleport will retry discovery automatically.

**Important:** The IAM Role must be configured in the AWS Organizations management account (or a delegated administrator account). These APIs can only be called from the management account, even with correct permissions.

**Note:** IAM permission changes may take a few minutes to propagate in AWS. Teleport also polls for EC2 instances periodically, so the fix may not be reflected immediately. This task will automatically resolve once discovery succeeds.
