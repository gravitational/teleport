# Missing AWS account permissions

Teleport failed to discover EC2 instances because the integration's IAM role lacks the required permission to discover AWS account resources.

This error can occur when using `regions: ["*"]` (wildcard) in the EC2 matcher configuration if the integration's IAM role lacks permission to list AWS regions. The wildcard configuration requires Teleport to call `account:ListRegions` to determine which regions to scan.

This error can also occur when the required `ec2:DescribeInstances` permission is missing, which Teleport needs to find EC2 instances that match your auto-discovery configuration.

**How to fix:**

Add the following permissions to the IAM role used by the integration:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "EC2Discovery",
      "Effect": "Allow",
      "Action": [
        "account:ListRegions",
        "ec2:DescribeInstances",
        "ssm:DescribeInstanceInformation",
        "ssm:SendCommand",
        "ssm:GetCommandInvocation",
        "ssm:ListCommandInvocations"
      ],
      "Resource": ["*"]
    }
  ]
}
```

These permissions allow Teleport to:

- `account:ListRegions` - Determine which AWS regions to scan (required when using `regions: ["*"]`)
- `ec2:DescribeInstances` - Find EC2 instances that match your auto-discovery configuration
- `ssm:DescribeInstanceInformation` - Check SSM agent status on discovered instances before installation
- `ssm:SendCommand` - Run the Teleport installation script on instances via SSM
- `ssm:GetCommandInvocation` - Retrieve installation script execution results from instances
- `ssm:ListCommandInvocations` - Poll for SSM command execution status during installation

**Alternative fix for account:ListRegions**

`account:ListRegions` is only required if `regions: ["*"]` is used.

If you prefer not to grant `account:ListRegions`, specify explicit region names in your matcher configuration instead of using `regions: ["*"]`. Explicitly listing regions means Teleport doesn't need to dynamically discover them via the API, thus not requiring that permission:

```yaml
regions:
  - us-east-1
  - us-west-2
  - eu-west-1
```

After applying the fix, mark this task as resolved. Teleport will retry discovery automatically.

**Note:** IAM permission changes may take a few minutes to propagate in AWS. Teleport also polls for EC2 instances periodically, so the fix may not be reflected immediately. This task will automatically resolve once discovery succeeds.
