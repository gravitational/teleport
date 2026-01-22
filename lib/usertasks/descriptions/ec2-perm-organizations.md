# Missing permissions: AWS Organizations
Teleport failed to discover EC2 instances because the integration's IAM role lacks permissions to query AWS Organizations.

This error occurs when using organization-wide discovery, which requires Teleport to list accounts across your AWS Organization.

**How to fix**

Add the following permissions to the IAM role used by the integration:

```json
{
  "Effect": "Allow",
  "Action": [
    "organizations:ListRoots",
    "organizations:ListChildren",
    "organizations:ListAccountsForParent"
  ],
  "Resource": "*"
}
```

These permissions allow Teleport to:
- `organizations:ListRoots` - Find the root of your organization
- `organizations:ListChildren` - Navigate organizational units (OUs)
- `organizations:ListAccountsForParent` - List accounts within each OU

After applying the fix, mark this task as resolved. Teleport will retry discovery automatically.
