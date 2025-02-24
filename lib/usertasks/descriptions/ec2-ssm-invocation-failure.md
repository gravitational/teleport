Teleport failed to access the SSM Agent to auto enroll the instance.
Some instances failed to communicate with the AWS Systems Manager service to execute the install script.

Usually this happens when:

**Missing policies**

The IAM Role used by the integration might be missing some required permissions.
Ensure the following actions are allowed in the IAM Role used by the integration:
- `ec2:DescribeInstances`
- `ssm:DescribeInstanceInformation`
- `ssm:GetCommandInvocation`
- `ssm:ListCommandInvocations`
- `ssm:SendCommand`

**SSM Document is invalid**

Teleport uses an SSM Document to run an installation script.
If the document is changed or removed, it might no longer work.