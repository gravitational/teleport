# Teleport AWS Terraform

This section of the Teleport GitHub repo contains AWS Terraform definitions for two Teleport cluster configurations.

- A simple starter Teleport cluster to, quickly and cost-effectively, demo or POC Teleport on a single node (auth, proxy, and node processes running on one t3.nano ec2 instance).
- A production worthy high-availability auto-scaling Teleport Cluster. This cluster makes use of several AWS technologies, provisioned and configured using Terraform.

If you are planning on using our Terraform example in production, please reference the high-availability auto-scaling Teleport Cluster for best practices. Our Production Guide outlines in-depth details on how to run Teleport in production.

## Prerequisites

We recommend familiarizing yourself with the following resources prior to reviewing our Terraform examples:

- [Teleport Architecture](https://goteleport.com/docs/reference/architecture/)
- [Admin Guide](https://goteleport.com/docs/admin-guides/management/admin/)

In order to spin up AWS resources using these Terraform examples, you need the following software:

- terraform v1.0+ [install docs](https://learn.hashicorp.com/tutorials/terraform/install-cli)
- awscli v1.14+ [install docs](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)

## Projects

- Starter Teleport Cluster
  - [Get Started](starter-cluster/README.md)

- HA Auto-Scaling Teleport Cluster
  - [Get Started](ha-autoscale-cluster/README.md)

## How to get help

If you're having trouble, check out our [GitHub Discussions](https://github.com/gravitational/teleport/discussions).

For bugs related to this code, please [open an issue](https://github.com/gravitational/teleport/issues/new/choose).

## Public Teleport AMI IDs

Please [see the AMIS.md file](AMIS.md) for a list of public Teleport AMI IDs that you can use.

## This is not the Teleport Terraform Provider

If you are looking for Teleport's [Terraform Provider](https://goteleport.com/docs/reference/terraform-provider/) which can be used to provision users, roles, auth connectors and other resources inside an existing Teleport cluster, its source code can be found here: https://github.com/gravitational/teleport/tree/master/integrations/terraform
