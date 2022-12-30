---
authors: STeve Huang (xin.huang@goteleport.com)
state: implemented
---

# RFD 67 - Redis database access in AWS ElastiCache and MemoryDB

## Required Approvers
* Engineering: @r0mant && @smallinsky
* Security: @reedloden

## What

Support Redis database access in AWS
[ElastiCache](https://aws.amazon.com/elasticache/redis/) and
[MemoryDB](https://aws.amazon.com/memorydb/).

## Why

Redis is one of the most popular in-memory databases, and Amazon offers AWS
managed solutions like ElastiCache and MemoryDB.

As described in [Database Access
RFD](https://github.com/gravitational/teleport/blob/master/rfd/0011-database-access.md),
mutual TLS is used between database clients and proxy, and between proxy and
database service. The same applies to Redis access.

However, neither mutual TLS nor IAM authentication is supported by Redis
managed by AWS. This document mainly focuses on access from database service to
Redis managed by AWS.

## Details

### In-transit encryption (TLS)

For security purposes, Teleport should ONLY
support databases with in-transit encryption (TLS) enabled.

[Enabling TLS on ElastiCache
Redis](https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/in-transit-encryption.html#in-transit-encryption-enable)
can be a complicated process, and all client applications must be updated at
the same time. Therefore, Teleport should NOT automatically enable TLS for
existing clusters. Redis servers without TLS are NOT supported, and users are
recommended to enable TLS separately to enable Teleport database access.

### Auth

Redis can be configured with one of the following authentication methods, and
only one at a time.

#### Auth - Redis with no auth

Redis can be configured without any auth method.

"default" user is always used in this configuration, and no `auth` command is
required upon a successful connection.

#### Auth - Redis with Redis Auth

[Redis Auth](https://redis.io/commands/auth/)
uses a single token/password that must be shared for all client applications.

"default" user is always used in this configuration, and users have to manually
input `auth` command upon a successful connection. <sup>1<sup>

#### Auth - Redis with ACL

[Redis ACL](https://redis.io/docs/manual/security/acl/) (also known as RBAC for
ElastiCache Redis) is introduced in Redis 6. Both ElastiCache and MemoryDB
provide APIs to manage database users and user groups for ACL.

As with all other databases, the database users should be created by "database
admins" (by users, not Teleport) to assign desired permissions. 

To provide better security than using static passwords, users can tag
ElastiCache and MemoryDB users with label **`teleport.dev/managed: true`** for
Teleport to manage the passwords for those database users.

Database service discovers database users by listing users from the ElastiCache
user group or MemoryDB ACL attached to the Redis database. For database users
discovered with label **`teleport.dev/managed: true`**, passwords are randomly
generated, periodically rotated (e.g. every 20 minutes plus jitter), and securely
stored in the secret store (more on the secret store in a later section).

The creation time of the current version of the password should always be
checked before applying a password rotation. If the current version is created
fairly recently (e.g. less than 15 minutes), it is most likely that the
password has been updated by another service in HA mode, so there is no action
required. If the current version is indeed "expired", the database service will
perform a rotation. In cases where more than one database service tries to
rotate the same user at the same time, the secret store implementation
guarantees only the first caller succeeds.

It has been found that it may take a few minutes for AWS to propagate user
password changes to the Redis servers. To work around this, two versions of the
passwords (`PREVIOUS` and `CURRENT`) are set to be effective AT THE SAME TIME
for the database user, and `PREVIOUS` version can be used to guarantee success.
For instance, a database user with passwords `v7` and `v8` is getting a
password rotation to use `v8` and `v9`, after `v9` is created in the secret
store. `v8` is a valid password while the password change is propagating or
after. (From another point of view, `v8` is the current password and `v9` is
the next.)

When a client tries to connect to ElastiCache and MemoryDB Redis servers, the
database service automatically logins Teleport managed database users by
sending `auth` commands with the stored passwords (using the `PREVIOUS` version
as described above) to the Redis server. For database users not managed by
Teleport, users can manually input `auth`.

### Secret store

#### Secret store - Interface

For ElastiCache/MemoryDB Redis usage, it is natural to use an AWS native
solution (aka [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/)).
Though other secret stores could be supported in the future. <sup>[2]<sup>

Here is a sample interface for managing secrets using the secret store.
```
// Value is the secret value.
type Value struct {
    // Key is the key path of the secret.
    Key string
    // Value is the value of the secret.
    Value string
    // Version is the version of the secret value.
    Version string
    // CreatedAt is the creation time of this version.
    CreatedAt time.Time
}

// Secrets defines an interface for managing secrets. A secret consists of a
// key path and a list of versions that hold copies of current or past secret
// values.
type Secrets interface {
    // Create creates the secret with the provided path and creates first
    // version with provided value.
    Create(ctx context.Context, key, value string) error

    // Delete deletes the secret with the provided path. All versions of the
    // secret are deleted at the same time.
    Delete(ctx context.Context, key string) error

    // PutValue creates a new secret version for the secret. CurrentVersion can
    // be provided to perform a test-and-set operation, and an error will be
    // returned if the test fails.
    PutValue(ctx context.Context, key, value, currentVersion string) error

    // GetValue returns the secret value for provided version. Besides version
    // string returned from PutValue, two specials versions "CURRENT" and
    // "PREVIOUS" can also be used to retrieve the current and previous
    // versions respectively. If the version is empty, "CURRENT" is used.
    GetValue(ctx context.Context, key, version string) (*Value, error)
}
```
It is up to the caller to call `PutValue` periodically to rotate the secret.

#### Secret store - AWS Secrets Manager

AWS Secrets Manager is used to store passwords for ElastiCache and MemoryDB
users. An optional `secret_store` block can be specified per database to
overwrite default settings.
```
db_service:
  enabled: "yes"

  databases:
  - name: "elasticache-example"
    protocol: "redis"
    uri: "master.example.xxxxxx.use1.cache.amazonaws.com"
    aws:
      region: "us-east-1"

      # Optional secret store settings.
      secret_store:
        # Secret key prefix. Defaults to "teleport/".
        # Secrets for Teleport managed ElastiCache users can be found at
        # "<key_prefix>/elasticache/<user-id>" in AWS Secrets Manager.
        key_prefix: "teleport/"

        # The ARN, key ID, or alias of the AWS KMS key that Secrets Manager
        # uses to encrypt the secret value. Defaults to `aws/secretsmanager`.
        kms_key_id: "my-kms-key"
```

The user must grant Teleport service the following IAM permissions to manage
the secrets:
```json
{
  "Version": "2012-10-17",
  "Statement": {
    "Effect": "Allow",
    "Action": [
      "secretsmanager:DescribeSecret",
      "secretsmanager:CreateSecret",
      "secretsmanager:UpdateSecret",
      "secretsmanager:DeleteSecret",
      "secretsmanager:GetSecretValue",
      "secretsmanager:PutSecretValue",
      "secretsmanager:TagResource",
    ],
    "Resource": "arn:{partition}:secretsmanager:{region}:{account-id}:secret:{key-prefix}*"
  }
}
```
Here `key-prefix` prevents Teleport from accessing user's other secrets in the
Secrets Manager. To manage a secret with a KMS key other than the default
`aws/secretsmanager`, `kms:GenerateDataKey` and `kms:Decrypt` to the key are
also required.

For racing `PutSecretValue` calls,
[`PutSecretValueInput.ClientRequestToken`](https://docs.aws.amazon.com/sdk-for-go/api/service/secretsmanager/#PutSecretValueInput)
can be used to "ensure idempotency" as AWS prevents different secret values of
the same `ClientRequestToken` to be written. When preparing `PutSecretValue`
call, an MD5 UUID can be generated from the version string of the CURRENT
value, and used for `ClientRequestToken`. When `PutSecretValue succeeds, the
`ClientRequestToken` becomes the version string of the latest value. This
effectively makes version strings rolling MD5 UUIDs of their previous versions,
and only the first call to create each version can succeed.

Staging labels `AWSCURRENT` and `AWSPREVIOUS` can be used to retrieve the
latest version and the previous version of the secret respectively.

Using AWS Secrets Manager does incur extra
[costs](https://aws.amazon.com/secrets-manager/pricing/) for users. As an
example, let's say that a Teleport cluster with three database agents is
managing one ElastiCache Redis user. The montly cost to store one secret is
$0.40. Assuming the secret is rotated 100 times per day, there will be about
100 put calls plus 3 * 100 get calls per secret per day. This sums to 12000 API
calls per month, which costs $0.06 ($0.05 for every 10000 calls). Therefore,
the total monthly cost will be about $0.46 for managing one ElastiCache user
with three database agents.

## Security

To reduce security risks by using passwords in backend services:
- Only well-known and attested secrets management tools like AWS Secrets
  Manager is used to store secrets securely.
- Generate random passwords using the maximum length available. In the case for
  Redis users, the random passwords are generated with length of 128 characters.
- Rotate passwords frequently. In the case using AWS Secrets Manager, password
  rotation is performed every 15~20 minutes.
- Provide the option use custom KMS key for secret encryption.

Lastly, permissions to all AWS resources used by Teleport services are granted
by users through AWS IAM.

## UX

### `tsh` client

The user experience when using the `tsh` client to connect Redis in AWS is the
same as connecting to self-hosted Redis (or other databases).

### Configure database service

Auto discovery is supported for both ElastiCache and MemoryDB databases.
Similar to existing RDS and Redshift auto discovery feature, very minimal
configuration is required to setup auto discovery.
```
# Example database service with elasticache auto discovery.
db_service:
  enabled: "yes"
  aws:
  - types: ["elasticache"]
    regions: ["ca-central-1"]
    tags:
      "vpc-id": "vpc-abcdef"
```

In addition to auto discovery, users can also manually configure an ElastiCache
or MemoryDB database with the ability to overwrite default settings (see
`elasticache-example` sample config in above section).

To provide proper IAM permissions required for ElastiCache, MemoryDB, and
SecretsManager, [Database access
configurator](https://github.com/gravitational/teleport/blob/master/rfd/0046-database-access-config.md)
is expanded to generate the required IAM policies.

### Teleport managed users

Users are asked to tag ElastiCache and MemoryDB users with a special label
**`teleport.dev/managed`** with value `true` while configuring database users.
Teleport managed users are periodically discovered at runtime so there is no
need to reconfigure or restart the database service.

## Alternatives considered

1. Rotating secrets for Redis with Redis Auth
  - Teleport can potentially take control of the shared token and rotate it for
    security. However, users must update all other Redis client applications to
    use tokens generated by Teleport, which might require a complicated
    migration process. Teleport might also need to provide APIs for retrieving
    secret key paths or values. This could be a potential feature in the
    future.

2. Potential secret store implementations:
  - [Password Rotation with HashiCorp
    vault](https://www.hashicorp.com/resources/painless-password-rotation-hashicorp-vault).
    (Note: `put` operation supports CheckAndSet.)
