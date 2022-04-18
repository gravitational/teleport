---
authors: Andrew Burke (andrew.burke@goteleport.com)
state: draft
---

# RFD X - Import EC2 Instance Tags

## What

Teleport nodes running on EC2 instances automatically add instance tags as labels.

### Related issues

- [#11627](https://github.com/gravitational/teleport/issues/11627)

## Why

The current recommended method of [using EC2 tags as Teleport labels](https://goteleport.com/docs/setup/guides/ec2-tags/) requires
- Creating a custom script to fetch the tags
- Individually adding each tag as a dynamic label
- Using the AWS API gateway, where the cost scales with the number of nodes using it

As of January 2022, [instance tags are available via the instance metadata service](https://aws.amazon.com/about-aws/whats-new/2022/01/instance-tags-amazon-ec2-instance-metadata-service/). This will allow Teleport nodes to discover their own instance tags. Unlike the AWS API gateway, instance metadata requests are free and per-instance.

## Details

EC2 tags will be supported everywhere that dynamic labels are currently supported (i.e. SSH, Kube, Apps, and Databases).

When a node is created, check if it is running in an EC2 instance. If it is, start a service (similar to dynamic labels) that periodically [every 10min] queries the instance metadata service and updates the tags. Tags created this way will use the [`aws`/`aws.ec2`/`aws/ec2`/`ec2`] prefix.

In order to use this feature, instance tags in metadata must be enabled for the instance.

### Special Tags

If the instance has the tag `Hostname` with a nonempty value, use that value as the node's hostname.
