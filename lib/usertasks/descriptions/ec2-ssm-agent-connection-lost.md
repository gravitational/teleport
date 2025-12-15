# SSM Agent lost connection
Auto enrolling EC2 instances requires the SSM Agent to be installed and running on them.
Some instances appear to have lost connection to Amazon Systems Manager.

You can see which instances lost connection using the [SSM Fleet Manager](https://console.aws.amazon.com/systems-manager/fleet-manager/managed-nodes).

The most common issues for instances losing connection:

**SSM Agent is not running**

Ensure the SSM Agent is running in the instance and is not reporting any error.
Please check the instructions [here](https://docs.aws.amazon.com/systems-manager/latest/userguide/ssm-agent-status-and-restart.html).

**SSM Agent can't reach the Amazon Systems Manager service**

Ensure the instance's security groups allows outbound connections to Amazon Systems Manager endpoints.
Allowing outbound on port 443 is enough for the agent to connect to AWS.

**Instance is missing IAM policy**

The SSM Agent requires the `AmazonSSMManagedInstanceCore` managed policy.
Ensure the instance has an IAM Profile and that it includes the above policy.
For more information please refer to [this page](https://docs.aws.amazon.com/systems-manager/latest/userguide/session-manager-getting-started-instance-profile.html).

After following the steps above, you can mark the task as resolved.
Teleport will try to auto-enroll these instances again.