# SSM - Teleport Agent installation failure

Teleport reached the EC2 instance via AWS SSM, but the Teleport agent installation failed.

Common causes:

- The instance already has an agent configured for a different Teleport cluster
- The Teleport binary could not be downloaded
- The SSM Agent version is below 3.1

Below you'll find two links: the Resource Name links to the EC2 instance details, and the Invocation URL provides details on the cause of the failure.

