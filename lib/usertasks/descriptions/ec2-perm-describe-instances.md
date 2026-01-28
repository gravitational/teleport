# Missing permission: ec2:DescribeInstances
Teleport failed to discover EC2 instances because the integration's IAM role lacks permission to describe EC2 instances.

The `ec2:DescribeInstances` permission is required for Teleport to find EC2 instances that match your auto-discovery configuration.

**How to fix**

Add the `ec2:DescribeInstances` permission to the IAM role used by the integration:

```json
{
  "Effect": "Allow",
  "Action": "ec2:DescribeInstances",
  "Resource": "*"
}
```

After applying the fix, mark this task as resolved. Teleport will retry discovery automatically.
