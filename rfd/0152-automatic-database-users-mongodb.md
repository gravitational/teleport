---
authors: STeve Huang (xin.huang@goteleport.com)
state: draft
---

# RFD 152 - Database Automatic User Provisioning for MongoDB

## Required approvers

Engineering: @r0mant || @smallinsky
Product: @klizhentas || @xinding33
Security: @reedloden || @jentfoo

## What

This RFD discusses on how to expand Database Automatic Provisioning feature for
MongoDB.

## Why

Automatic User Provisioning has been implemented for several SQL databases,
including PostgreSQL and MySQL, with the basic design described in [RFD
113](https://github.com/gravitational/teleport/blob/master/rfd/0113-automatic-database-users.md).

Adding support for database user provisioning to MongoDB presents unique
challenges due to differences in architecture compared to traditional SQL
databases.

This RFD aims to identify the challenges and provide solutions to address them.

Since the differences in architecture between MongoDB Atlas and self-hosted
MongoDB are significant, they will be discussed separately in this RFD.

## MongoDB Atlas Details

Automatic User Provisioning will NOT be supported for MongoDB Atlas with the
reasons discussed below. This RFD should be updated if better solutions are
found in future iterations.

[Database
Users](https://www.mongodb.com/docs/atlas/security-add-mongodb-users/) and
[Custom Database
Roles](https://www.mongodb.com/docs/atlas/security-add-mongodb-roles/) for
MongoDB Atlas are managed at a Atlas project level.

As a consequence, database users and roles are not modifiable through
in-database connections.

Instead, one can authenticate with Atlas using the Atlas SDK and use APIs to
manage these database users and roles. Multiple deployment jobs will be created
to update the MongoDB clusters in this project, upon successful APIs calls.

In my personal testing on an Atlas project with a single MongoDB cluster, it
takes 10~20 seconds for the deployment job to refresh the database user in the
target MongoDB cluster.

With the current design of Automatic User Provisioning, the database user must
be updated with new role assignments before and after client connection.
However, waiting for 10+ seconds for provisioning the database user each
connection will result in a very bad user experience (also client may just time
out).

In addition, since the database user is shared across all MongoDB clusters
within the project, the auto-provisioned database user will gain unintentional
access to other MongoDB clusters that are not currently being connected through
Teleport.

Last but not the least, if we were to implement this, we must manage all API
keys used for authenticating different projects on Atlas.

## Self-hosted MongoDB Details

The overwall flow and logic wil follow the previous [RFD
113](https://github.com/gravitational/teleport/blob/master/rfd/0113-automatic-database-users.md).
All differences will be outlined in the sections below.
