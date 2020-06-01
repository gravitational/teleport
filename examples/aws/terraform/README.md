# Teleport AWS Terraform

This section of the Teleport Github repo contains AWS Terraform definitions for two Teleport cluster configurations.

- A simple starter Teleport cluster to, quickly and cost-effectively, demo Teleport on a single node (auth, proxy, and node processes running on one t3.nano ec2 instance).
- A production high-availability auto scaling Teleport Cluster. This cluster makes use of several AWS technologies, provisioned and configured using Terraform.

If you are planning on using these Terraform definitions in production, please reference the production high-availability auto scaling Teleport Cluster.

## Prerequisites

We recommend familiarizing yourself with the following resources prior to reviewing our Terraform code:

- [Teleport Architecture](https://gravitational.com/teleport/docs/architecture/teleport_architecture_overview/)
- [Admin Guide](https://gravitational.com/teleport/docs/admin-guide/)

In order to spin up AWS resources using these Terraform examples, you need the following software:

- terraform v0.12+ ([install docs](https://learn.hashicorp.com/terraform/getting-started/install.html))
- awscli v1.14+ ([install docs](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html))

## Beginner: Starter Teleport Cluster

> Note: We do not recommend using this configuration for production environments. This is a simplified Teleport configuration for demo and educational purposes. Please use our HA Auto Scaling Teleport Cluster for production configurations.

Start Here: [README](starter-cluster/README.md)

## Advanced: HA Auto Scaling Teleport Cluster

Start Here: [README](ha-autoscale-cluster/README.md)

## How to get help

If you're having trouble, check out our Discourse community. <https://community.gravitational.com>

For bugs related to this code, please [open an issue](https://github.com/gravitational/teleport/issues/new/choose).
