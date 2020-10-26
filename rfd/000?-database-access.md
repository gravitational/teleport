---
authors: Roman Tkachenko (roman@gravitational.com)
state: draft
---

# RFD 000? - Teleport Database Access (Preview)

## What

This document discusses high-level design points, user experience and some
implementation details of the Teleport Database Access feature.

_Note: This document refers to an early preview of the Database Access feature 
and covers functionality that will be available in the initial release._

With Teleport Database Access users can:

- Provide secure access to databases without exposing them over the public
  network through Teleport's reverse tunnel subsystem.
- Control access to specific database instances as well as individual
  databases and database users through Teleport's RBAC model.
- Track individual users access to databases as well as query activity
  through Teleport's audit log.

## Use cases

The feature is being developed with the following use-cases in mind.

### Human access

Users should be able to access the databases connected to Teleport using
regular database clients they normally use to connect directly such as
CLI clients (`psql`, `mysql`, etc.) as well as graphical interfaces (`pgAdmin`,
`MySQL Workbench`, etc.).

The use-case for this is to grant users access to a database in a transparent
fashion, for example to let them do development in a test/stage environment
or perform an emergency recovery on a production database instance using
familiar tools.

### Robot access

The feature should be automation friendly so existing CI systems can take
advantage of it.

An example would be letting the tools like Ansible or Drone perform routine
actions on a database such as migrations or backups and be able to audit it.

### Programmatic access

Programmatic access, as in configuring an application to talk to a database
through Teleport proxy, should work automatically as long as it uses a driver
that properly implements a particular database protocol and supports mutual
TLS authentication.

However, it is not the primary use-case, at least for the initial release,
since it comes with a number of additional concerns and considerations such
as performance requirements for high-traffic applications, automatic failover
and so on.

## Scope

For the initial release we're focusing on supporting a single type of the
database, PostgreSQL, with full protocol parsing.

The following PostgreSQL deployment models are supported:

* PostgreSQL instances deployed on-premises.
* AWS RDS for PostgreSQL.
* PostgreSQL-compatible AWS Aurora.

The following features are provided:

* Connecting to the database through the Teleport proxy, incl. trusted
  clusters support.
* Limiting access to database instances by labels with Teleport roles.
* Limiting access to individual databases (within a particular database
  instance) and database users.
* Auditing of database connections and executed queries.

## Architecture

Architecturally Database Access is very similar to [Application Access](0008-application-access.md).
Teleport gains a new mode of operation, "database service", similar to "ssh
service", "application service" or a "kube service".

Database service runs inside the customer private network and proxies database
client connections received from the Teleport proxy to the target database. It
is also responsible for protocol parsing of supported databases and authorizing
requests coming from the proxy (i.e. granting/denying access to particular
databases/database users).

```
              |‾‾‾‾‾‾‾|  reverse  |‾‾‾‾‾‾‾‾‾‾‾‾|          |‾‾‾‾‾‾‾‾‾‾|
psql -------> | proxy | <-------- | db service | -------> | postgres |
              |_______|   tunnel  |____________|          |__________|
```

The database client (such as psql) will talk to Teleport web proxy port (`3080`
by default) which will use multiplexing to detect the database protocol and
dispatch to an appropriate service.

## Authentication

In this model, there are 3 points where authentication needs to happen.

### Database client <---> proxy

**Mutual TLS.** Database clients, such as psql, will use short-lived x509
certificates issued by Teleport in order to authenticate with the proxy.

The certificate metadata includes (via extensions) routing information about
the target database which users log into using `tsh db login` command (see
below for UX).

### Proxy <---> database service

**Mutual TLS.** The connection that is passed from proxy to a database service
is upgraded to TLS as well in order to be able to pass identity information
over the reverse tunnel.

In order to do that, proxy will generate an ephemeral certificate signed
by the user authority and re-encode all identity and routing information
extracted from the certificate supplied by the client into it.

### Database service <---> database instance

**Mutual TLS (onprem).**  When connecting to an onprem database instance, the
database service will use a client certificate for authentication, which it
will generate on the fly according to the database requirements (for example,
PostgreSQL requires the database user name to be encoded in the certificate
as common name).

This also means that the database needs to be configured with Teleport's
certificate authority and (in general case) use server certificate issued by
Teleport which can be done using `tctl auth sign` command (see below for UX).

**Password auth (RDS/Aurora).** Amazon RDS/Aurora don't support client cert
authentication. Instead, they support IAM authentication so Teleport database
service will generate a short-lived authentication token using RDS API that
will be used as a password:

https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.html

In order to be able to verify RDS/Aurora server certificate, users will need
to download RDS root certificate and configure the database service to trust
it:

https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL.html

## Configuration

### Teleleport database service

The following new configuration section is added to the Teleport file config:

```yaml
# New global key housing the database service configuration.
db_service:
  # Enable or disable the database service.
  enabled: "yes"
  # List of the database this service is proxying.
  databases:
    # Database instance name, used to refer to an instance in CLI like tsh.
  - name: "postgres-prod"
    # Optional free-form verbose description of a database instance.
    description: "Production instance of PostgreSQL 13.0"
    # Database procotol, only "postgres" is supported initially.
    protocol: "postgres"
    # Database connection endpoint, should be reachable from the service.
    endpoint: "postgres.internal.example.com:5432"
    # Optional CA certificate path, e.g. for AWS RDS/Aurora.
    ca_path: "/path/to/root.pem"
    # AWS specific configuration for RDS/Aurora databases.
    aws:
      # Use IAM authentication with RDS/Aurora database.
      auth: "iam"
      # Optional AWS region RDS/Aurora database is running in.
      region: "us-east-1"
      # Optional AWS access key, will be taken from environment if unspecified.
      access_key: "XXX"
      # Optional AWS secret key, will be taken from environment if unspecified.
      secret_key: "YYY"
    # Static labels assigned to the database instance, used in RBAC.
    labels:
      env: "stage"
    # Dynamic labels assigned to the database instance, used in RBAC.
    commands:
    - name: "time"
      command: ["date", "+%H:%M:%S"]
      period: "1m"
```

*TODO(r0mant): Add information about starting db service using tsh join.*

#### RDS/Aurora

When configuring Teleport database service with RDS/Aurora databases, the
following fields need to be set:

```yaml
db_service:
  enabled: "yes"
  databases:
  - name: "postgres-rds"
    protocol: "postgres"
    endpoint: "postgres-rds.xxx.us-east-1.rds.amazonaws.com:5432"
    ca_path: "/opt/rds/rds-ca-2019-root.pem"
    aws:
      auth: "iam"
      region: "us-east-1"
```

* `endpoint` is the endpoint for a particular database from RDS control panel.
For Aurora, it can be any of the exposed database endpoints, reader or writer.
* `ca_path` is a path to the [RDS root certificate](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.SSL.html).
* `aws.auth` is the authentication method, only "iam" is supported.
* `aws.region` is the region the database is deployed in.

AWS credentials, if left unspecified, will be retrieved from the environment
in the same way as `aws` CLI tool does, i.e. from the credentials file or
supported environment variables. Credentials can also be provided in the 
service configuration via `aws.access_key` and `aws.secret_key` fields.

### Database instance

#### Onprem

To support mutual TLS auth, onprem database instance needs to be configured
with Teleport's certificate authority and (optionally) cert/key pair issued
by Teleport. The existing `tctl auth sign` command is used to produce these
secrets:

```sh
$ tctl auth sign --format=db --host=localhost --out=server --ttl=8760h
$ ls
server.cas server.crt server.key
```

Note that the `--host` parameter should match the hostname of the endpoint
the database will be connected at.

The certificate is signed by Teleport host authority which is intended for 
machine-to-machine communication.

*TODO(r0mant): Instead of using host authority, introduce another authority
specifically for db access? Would also help decouple authority rotation for
databases from the rest of the cluster.*

The generated secrets are then used to configure the database. For example,
for PostgreSQL:

```sh
$ cat postgresql.conf | grep ssl
ssl = on
ssl_ca_file = '/path/to/server.cas'
ssl_cert_file = '/path/to/server.crt'
ssl_key_file = '/path/to/server.key'
```

In addition, PostgreSQL access configuration file should be configured to 
require client certificate authentication:

```sh
$ cat pg_hba.conf
# TYPE  DATABASE        USER            ADDRESS                 METHOD
hostssl all             all             ::/0                    cert
hostssl all             all             0.0.0.0/0               cert
```

#### RDS/Aurora

There are a few things that need to be done in order to configure RDS/Aurora
database to support IAM authentication.

1. Enable IAM authentication when provisioning a new database, or on an
existing one. By default, only password authentication is enabled.

https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.Enabling.html

2. Configure IAM policy in order to allow the user Teleport database service
will use to connect to the database instance with the auth token. An example
policy is shown below.

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "rds-db:connect",
            "Resource": [
                "arn:aws:rds-db:us-west-1:1234567890:dbuser:db-ABCDEFGHIJKLMNOP/developer",
            ]
        }
    ]
}
```

Note that the above rule allows IAM authentication to a single database user,
it also supports wildcards.

https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.IAMPolicy.html

3. Grant `rds_iam` role to the database users. For example, for PostgreSQL:

```sql
CREATE USER developer; 
GRANT rds_iam TO developer;
```

https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/UsingWithRDS.IAMDBAuth.DBAccounts.html

## RBAC

Teleport role resource gets 3 new fields that allow to control access to 
database instances as well as individual databases and database users.

```yaml
kind: role
version: v3
metadata:
  name: developer
spec:
  allow:
    # Label selectors for database instances this role has access to.
    db_labels:
      environment: ["dev", "stage"]
    # Database names (within a database instance) this role has access to.
    db_names: ["main", "metrics", "postgres"]
    # Database users this role can log in as.
    db_users: ["alice", "bob"]
```

## CLI

To connect to a database instance, a user first must login using regular
`tsh login` command:

```sh
$ tsh login --proxy=localhost:3080 --user=admin
```

Once logged in, they can see all available database using the following
command:

```sh
$ tsh db ls
Name            Description     Endpoint                                Labels
--------------- --------------- --------------------------------------- ---------
postgres        PostgreSQL 13.0 localhost:5432                          env=dev
postgres-rds    PostgreSQL 12.4 postgres-rds.xxx.rds.amazonaws.com:5432 env=stage
postgres-aurora PostgreSQL 11.6 postgres-rds.yyy.rds.amazonaws.com:5432 env=prod
```

The list will only show databases the logged in user has access to (as per 
the database labels). For example, developer role shown above won't be able 
to see the Aurora database instance:

```sh
$ tsh login --proxy=localhost:3080 --user=dev
$ tsh db ls
Name            Description     Endpoint                                Labels
--------------- --------------- --------------------------------------- ---------
postgres        PostgreSQL 13.0 localhost:5432                          env=dev
postgres-rds    PostgreSQL 12.4 postgres-rds.xxx.rds.amazonaws.com:5432 env=stage
```

To log into a specific database instance, a user executes `tsh db login`
command. This will retrieve the certificate with encoded database information
in it:

```sh
$ tsh db login postgres
```

The `tsh status` command shows the name of the database a user is logged into:

```sh
$ tsh status
> Profile URL:        https://127.0.0.1:3080
  Logged in as:       dev
  Roles:              db-developer*
  ...
  Database:           postgres
  ...
```

The `tsh db login` command also prints a footer explaining how to connect
to the database:

```sh
Connection information for "postgres" has been saved to ~/.pg_service.conf.
You can connect to the database using the following command:

  $ psql "service=postgres user=<user> dbname=<dbname>"

Or configure environment variables and use regular CLI flags:

  $ eval $(tsh db env)
  $ psql -U <user> <database>
```

Upon successful login to a PostgreSQL instance, `tsh` configures a [connection
service file](https://www.postgresql.org/docs/9.1/libpq-pgservice.html) which
resides in `~/.pg_service.conf`.

The service file contains connection information so PostgreSQL clients can
refer to it using `service` connection string parameter which all libpq-based
clients should recognize (e.g. `psql` and `pgAdmin` do). The service file
has ini format and has the following information:

```ini
[postgres]
host=127.0.0.1
port=3080
sslmode=verify-full
sslrootcert=/home/user/.tsh/keys/127.0.0.1/certs.pem
sslcert=/home/user/.tsh/keys/127.0.0.1/dev-x509.pem
sslkey=/home/user/.tsh/keys/127.0.0.1/dev
```

Users can connect to the database using the following command:

```sh
$ psql "service=postgres user=developer dbname=metrics"
```

Alternatively, `tsh` can output a set of environment variables supported by
PostgreSQL clients for users to set in their session:

```sh
$ tsh db env
export PGHOST=127.0.0.1
export PGPORT=3080
export PGSSLMODE=verify-full
export PGSSLROOTCERT=/home/user/.tsh/keys/127.0.0.1/certs.pem
export PGSSLCERT=/home/user/.tsh/keys/127.0.0.1/dev-x509.pem
export PGSSLKEY=/home/user/.tsh/keys/127.0.0.1/dev
$ eval $(tsh db env)
$ psql -U developer metrics
```

## Audit events

The following new events are emitted to the Teleport audit log for database
sessions.

* `db.session.start` fires upon successful connection to the database.

```json
{
  "code": "TDB00I",
  "db_database": "postgres",
  "db_endpoint": "localhost:5432",
  "db_name": "postgres",
  "db_protocol": "postgres",
  "db_user": "postgres",
  "ei": 0,
  "event": "db.session.start",
  "namespace": "default",
  "server_id": "5603cd43-1172-4f1f-8e35-2bf74dfecf15",
  "sid": "427abfa3-fb1a-4a55-879c-77f5b6257cdb",
  "time": "2020-11-06T03:50:20.802Z",
  "uid": "f12b5199-5a1f-4e48-af4f-6980afffc48e",
  "user": "dev"
}
```

* `db.session.end` fires upon database connection termination.

```json
{
  "code": "TDB01I",
  "db_database": "test",
  "db_endpoint": "localhost:5432",
  "db_name": "postgres",
  "db_protocol": "postgres",
  "db_user": "postgres",
  "ei": 0,
  "event": "db.session.end",
  "sid": "56335587-0157-4228-9556-aaa416741ef7",
  "time": "2020-11-06T03:50:20.802Z",
  "uid": "49e5f882-40ea-4bdb-b462-4cccf3536ada",
  "user": "dev"
}
```

* `db.query` fires when a database query is executed.

```json
{
  "code": "TDB02I",
  "db_database": "test",
  "db_endpoint": "localhost:5432",
  "db_name": "postgres",
  "db_protocol": "postgres",
  "db_query": "SELECT 1;",
  "db_user": "postgres",
  "ei": 0,
  "event": "db.query",
  "sid": "a53b0e1c-42f0-43ad-bbdd-9f3e07b54c05",
  "time": "2020-11-06T03:36:06.233Z",
  "uid": "7af4ea5d-ba1a-42d0-96a6-0c41b9c89bb0",
  "user": "dev"
}
```

## TODOs

* Trusted clusters.
* CA rotation.
