---
title: Teleport Database Access
description: Secure and Audited Access to Postgres Databases. Documentation to outline our preview.
---

# Teleport Database Access Preview

Teleport Database Access allows organizations to use Teleport as a proxy to
provide secure access to their databases while improving both visibility and
access control.

To find out whether you can benefit from using Database Access, see if you're
facing any of the following challenges in your organization:

* Do you need to protect and segment access to your databases?
* Do you need provide SSO and auditing for the database access?
* Do you have compliance requirements for data protection and access monitoring
  like PCI/FedRAMP?

If so, Database Access might help you solve some of these challenges.

## Features

With Database Access users can:

* Provide secure access to databases without exposing them over the public
  network through Teleport's reverse tunnel subsystem.
* Control access to specific database instances as well as individual
  databases and database users through Teleport's RBAC model.
* Track individual users' access to databases as well as query activity
  through Teleport's audit log.

## Diagram

The following diagram shows an example Database Access setup:

* Root cluster provides access to an onprem instance of PostgreSQL.
* Leaf cluster, connected to the root cluster, provides access to an
  onprem instance of MySQL and PostgreSQL-compatible AWS Aurora.
* Node connects another on-premise PostgreSQL instance (perhaps, a
  metrics database) via tunnel to the root cluster.

![Teleport database access diagram](../img/dbaccess.svg)

!!! info
    Teleport Database Access is currently under active development, with a
    preview release slated for Teleport 5.1 in December 2020. The preview
    will include support for PostgreSQL, including Amazon RDS and Aurora.

## Setup

Let's setup a sample Teleport Database Access deployment.

We will create a Teleport cluster and connect a PostgreSQL-flavored AWS Aurora
database to it via Teleport database proxy.

### Setup Aurora

Teleport Database Access uses IAM authentication with AWS RDS and Aurora
databases which can be enabled with the following steps.

#### Enable IAM Authentication

Open [Amazon RDS console](https://console.aws.amazon.com/rds/) and create a new
database instance with IAM authentication enabled, or modify an existing one to
turn it on. Make sure to use PostgreSQL database type.

See [Enabling and disabling IAM database authentication](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/UsingWithRDS.IAMDBAuth.Enabling.html)
for more information.

#### Create IAM Policy

To allow Teleport database service to log into the database instance using auth
token, create an IAM policy and attach it to the user whose credentials the
database service will be using, for example:

```json
{
   "Version": "2012-10-17",
   "Statement": [
      {
         "Effect": "Allow",
         "Action": [
             "rds-db:connect"
         ],
         "Resource": [
             "arn:aws:rds-db:us-east-2:1234567890:dbuser:cluster-ABCDEFGHIJKL01234/*"
         ]
      }
   ]
}
```

See [Creating and using an IAM policy for IAM database access](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/UsingWithRDS.IAMDBAuth.IAMPolicy.html)
for more information.

#### Create Database User

Database users must have a `rds_iam` role in order to be allowed access. For
PostgreSQL:

```sql
CREATE USER alice;
GRANT rds_iam TO alice;
```

See [Creating a database account using IAM authentication](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/UsingWithRDS.IAMDBAuth.DBAccounts.html)
for more information.

### Install Teleport

First, head over to the Teleport [downloads page](https://gravitational.com/teleport/download/)
and download the latest version of Teleport.

!!! warning
    As of this writing, no Teleport release with Database Access has been
    published yet.

Follow the installation [instructions](https://gravitational.com/teleport/docs/installation/).

### Start Teleport

Create a configuration file for a Teleport service that will be running
auth and proxy servers:

```yaml
teleport:
  data_dir: /var/lib/teleport
  nodename: test
auth_service:
  enabled: "yes"
  cluster_name: "test"
  listen_addr: 0.0.0.0:3025
  tokens:
  - proxy,node,database:qwe123
proxy_service:
  enabled: "yes"
  listen_addr: 0.0.0.0:3023
  web_listen_addr: 0.0.0.0:3080
  tunnel_listen_addr: 0.0.0.0:3024
  public_addr: teleport.example.com:3080
ssh_service:
  enabled: "no"
```

Start the service:

```sh
$ teleport start --debug --config=/path/to/teleport.yaml
```

### Start Teleport Database Service

Create configuration file for a database service:

```yaml
teleport:
  data_dir: /var/lib/teleport-db
  nodename: test
  # Auth token to connect to the auth server.
  auth_token: qwe123
  # Proxy address to connect to.
  auth_servers:
  - teleport.example.com:3080
db_service:
  enabled: "yes"
  databases:
    # Name of the database proxy instance, used to reference in CLI.
  - name: "postgres-aurora"
    # Free-form description of the database proxy instance.
    description: "AWS Aurora instance of PostgreSQL 13.0"
    # Database protocol.
    protocol: "postgres"
    # Database address.
    uri: "postgres-aurora-instance-1.xxx.us-east-1.rds.amazonaws.com:5432"
    # AWS specific configuration.
    aws:
      # Region the database is deployed in.
      region: us-east-1
    # Labels to assign to the database, used in RBAC.
    labels:
      env: dev
auth_service:
  enabled: "no"
ssh_service:
  enabled: "no"
proxy_service:
  enabled: "no"
```

Start the service:

```sh
$ teleport start --debug --config=/path/to/teleport-db.yaml
```

### Login

After the setup, log into the cluster and see the registered database proxy:

```sh
$ tsh login --proxy=teleport.example.com:3080
$ tsh db ls
```

### Connect

Log into the database and connect using `psql`:

```sh
$ tsh db login postgres
$ psql "service=postgres-aurora user=<db-user> database=<db-name>"
```

## Demo

<video autoplay loop muted playsinline controls style="width:100%">
  <source src="https://goteleport.com/teleport/videos/database-access-preview/dbaccessdemo.mp4" type="video/mp4">
  <source src="https://goteleport.com/teleport/videos/database-access-preview/dbaccessdemo.webm" type="video/webm">
Your browser does not support the video tag.
</video>


## RFD

Please refer to the [RFD document](https://github.com/gravitational/teleport/blob/roman/rfd/dba/rfd/0011-database-access.md)
for a more in-depth description of the feature scope and design.

## Feedback

We value your feedback. Please schedule a Zoom call with us to get in-depth
demo and give us feedback using [this](https://calendly.com/benarent/teleport-database-access?month=2020-11)
link.

If you found a bug, please create a [Github
Issue](https://github.com/gravitational/teleport/issues/new/choose).
