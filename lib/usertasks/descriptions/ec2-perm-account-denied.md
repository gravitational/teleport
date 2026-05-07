# Missing AWS account permissions

Teleport failed to discover EC2 instances because the integration's IAM role lacks the required permissions to discover AWS account resources.

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

After applying the fix, mark this task as resolved. Teleport will retry discovery automatically.

**Note:** IAM permission changes may take a few minutes to propagate in AWS. Teleport also polls for EC2 instances periodically, so the fix may not be reflected immediately. This task will automatically resolve once discovery succeeds.
