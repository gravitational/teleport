# EC2 Join Failure

Teleport was installed on the EC2 instance but the agent failed to join the cluster.

The service started and attempted the IAM join handshake but was rejected by the auth server. Check the **Audit Log** for the corresponding `ssm.run` event to see the captured journal output, or click the **Invocation Link** to view the output in the AWS SSM console.

Common causes and fixes:

**Invalid or nonexistent join token**

The token name in the discovery config does not match any token in the cluster. Verify the token exists (`tctl get token/<name>`), has not expired, and the name in the discovery config matches exactly.

**IAM ARN mismatch in token allow rules**

The token exists but its `allow` rules do not match the EC2 instance's identity. A token can match by AWS account ID alone (`spec.allow[].aws_account`) or by IAM role ARN (`spec.allow[].aws_arn`). The instance joins as `arn:aws:sts::<account>:assumed-role/<role-name>/<instance-id>`. If the token specifies `aws_arn`, the pattern must match the instance's assumed role. Verify the token's allow rules with `tctl get token/<name>`. See [Create the AWS joining token](https://goteleport.com/docs/enroll-resources/agents/aws-iam/#step-25-create-the-aws-joining-token).

**Network connectivity**

The instance cannot reach the Teleport Proxy. Ensure security groups and network ACLs allow outbound HTTPS to the proxy public address. Note that a successful package install does not guarantee proxy reachability, as packages are downloaded from external repositories (cdn.teleport.dev). If the instance requires an HTTP proxy to reach external services, ensure the Teleport systemd unit is configured to use it via its environment file. See [HTTP CONNECT proxies](https://goteleport.com/docs/reference/deployment/networking/#http-connect-proxies).
