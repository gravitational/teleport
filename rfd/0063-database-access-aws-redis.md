---
authors: STeve Huang (xin.huang@goteleport.com)
state: draft
---

# RFD 63 - Redis database access in AWS ElatiCache and MemoryDB

## What

Support Redis database access in AWS [ElatiCache](https://aws.amazon.com/elasticache/redis/) and [MemoryDB](https://aws.amazon.com/memorydb/).

## Why

Redis is one of the most popular in-memory databases, and Amazon offers AWS managed solutions like ElatiCache and MemoryDB.

As described in [Database Access RFD](https://github.com/gravitational/teleport/blob/master/rfd/0011-database-access.md), mutual TLS is used between database clients and proxy, and between proxy and database service. The same applies for Redis access.

However, neither mutual TLS nor IAM authentication is supported by Redis managed by AWS. This document mainly focus on access from database service to the Redis server.

## Details

### In-transit encryption (TLS)
For security purpose, Teleport should ONLY support databases with in-transit encryption (TLS) enabled.

[Enabling TLS on ElastiCache Redis](https://docs.aws.amazon.com/AmazonElastiCache/latest/red-ug/in-transit-encryption.html#in-transit-encryption-enable) can be a complicated process, and all client applications must be updated at the same time. Therefore, Teleport should NOT automatically enable TLS for existing clusters. Redis servers without TLS are "skipped" by database service, and users are recommended to enable TLS separately in order to enable Teleport database access.

For clusters with TLS enabled, Teleport performs full certificate validation at connection.

### Auth
Redis can be configured with one the following authentication methods, and only one at a time.

#### Redis with ACL
[Redis ACL](https://redis.io/docs/manual/security/acl/) (also known as RBAC for ElatiCache Redis) is introduced in Redis 6. Both ElatiCache and MemoryDB provide APIs to manage database users and user groups for ACL.

As with all other databases, the database users should be created by "database admins" (by users, not Teleport) to assign desired permissions. 

When an user runs `tsh db connect --db-user <database-user> <aws-rediaws-redis-server>`, database agent automatically sends `auth <database-user> <password>` to Redis server upon succesfull connection, to prevent user from entering the password manually.

To provide better security than using a static password, Teleport must take control of these database users and apply periodic password rotation (more details on secret store in later section). Users can separate the database users used by Teleport vs other client applications.

Here is a sample database service config to specify which database users to manage:
```
db_service:
  enabled: "yes"

  aws:
  # Discovers ElastiCache/MemoryDB Redis databases.
  - types: ["elasticache", "memorydb"]
    regions: ["us-west-1"]
    tags:
      "vpc-id": "xyz"

  # Manage (e.g. rotate password for) database users matching provided tags.
  - types: ["elasticache-user", "memorydb-user"]
    regions: ["us-west-1"]
    tags:
      "managed-by-teleport": "true"

  # Statically list database users (as alternative to using tags matching).
  users:
  - arn: "arn:aws:elasticache:ca-central-1:123456789000:user:steve-teleport"

```
The database service creates one secret with random password per database user, rotates it frequently (e.g. 15 minutes), and updates the database user with new password value using AWS APIs.

When database service tries to establish connection with Redis server, the password is retrieved from the secret store to perform `auth <database-user> <password>`. If the secret does not exist or expired, the connection is aborted. 

#### Redis with Redis Auth
[Redis Auth](https://redis.io/commands/auth/) uses a single token/password that must be shared for all client applications.

Teleport does NOT support Redis configured in this state, and users are recommended to migrate the database to use ACL/RBAC.<sup>[1]</sup>

#### Redis with no auth
Redis can also be configured without any auth method. "default" user is always used in this configuration, and no `auth` command is required upon succesfull connection.

### Secret Store
Using a password is undesirable but necessary in situation like this. For ElastiCache/MemoryDB Redis usage, it is natural to use a AWS native solution (aka [AWS Secrets Manager](https://aws.amazon.com/secrets-manager/)). Though other secret stores could be supported in the future. <sup>[2]<sup>

Here is a sample config for specifying the secret store in database service.
```
db_service:
  ...

  secert_store:
    type: "aws-secrets-manager"
    key_prefix: "teleport/"
```

Here is a sample interface for managing secrets using the secret store.
```
// PutValueRequest defines the put secret value request.                                
type PutValueRequest struct {                                
    // Key is the key path of the secret.                                
    Key string                                                       
    // Value is the value of the secret. A random value is generated if empty.                                
    Value string                                                     
    // RandomValueLength is the length of the random value to generate.                                
    RandomValueLength int                                
}                                
                                
// SecretValue is the secret value.                                
type SecretValue struct {                                
    // Key is the key path of the secret.                                
    Key string                                
    // Value is the value of the secret.                                
    Value string                                
    // Version is the version ID of the secret value.                                
    Version string                                
    // Created is the creation time of this version.                                
    Created time.Time                                
}                                
                                
// Secrets defines an interface for handling secrets. A secret consists of a                                
// key path and a list of versions which hold copies of current or past secret                                
// values.                                
type Secrets interface {                                
    // PutValue creates a new secret version for the secret. The secret is also                                
    // created if not exist.                                
    PutValue(req PutValueRequest) (SecretValue, error)                                
                                
    // GetValue returns the secret value for provided version. Besides version                                
    // ID returned from PutValue, two specials versions "CURRENT" and                                 
    // "PREVIOUS" can also be used to retrieve the current and previous                                         
    // versions respectively. If version is empty, "CURRENT" is used.                                                  
    GetValue(key string, version string) (SecretValue, error)                                                          
                                
    // Delete deletes a secret for provided path.                                
    Delete(key string) error                                
}
```
It is up to the caller to call `PutValue` periodically to rotate the secret.

#### Random password generation
[sethvargo/go-password](https://github.com/sethvargo/go-password) is used to generate random passwords. <sup>[3]<sup>

A minimum length of 16 should be enforced for generating random passwords. A mix of lower case letters, upper case letters, digits, and symobols should be also enforced.

#### AWS Secrets Manager
With `iam:PutUserPolicy` and `iam:PutRolePolicy` permissions, Teleport service can grant required permissions to itself. Users can limit the permissions by setting the following permissions boundaries (preferably using bootstrap configurator):
```json
{
  "Version": "2012-10-17",
  "Statement": {
    "Effect": "Allow",
    "Action": [
      "secretsmanager:CreateSecret",
      "secretsmanager:GetSecretValue",
      "secretsmanager:PutSecretValue",
      "secretsmanager:DeleteSecret",
    ],
    "Resource": "arn:aws:secretsmanager:{region}:{account-id}:secret:{key-prefix}*"
  }
}
```
`key_prefix` is used to prevent Teleport from accessing other secrets in the AWS Secrets Manager.

Secrets created by Teleport should have proper tags for easier cleanups (e.g. "teleport:service" -> "database", "teleport:createdby" -> "<hostid>").

Some limitations/considerations when using AWS Secrets Manager: <sup>[4]<sup>
- Secrets, by default, are bound to a single AWS region.
- A secret can have at most [100 versions per day](https://docs.aws.amazon.com/secretsmanager/latest/userguide/reference_limits.html) (about one version per 15 minutes). 
- Secrets are encrypted with KMS keys. By default, `aws/secretsmanager` (managed by AWS) is used.

## Alternatives considered
1. Supporting Redis with Redis Auth
  - Teleport can potentially take control of the shared token and rotate it for security. However, users must update all other Redis client applications to use tokens generated by Teleport, which might require a complicated migration process. Teleport might also need to provide APIs for retrieving secret key paths or values. This could be a potential feature in the future.

2. Potential secret store implementations:
  - [Password Rotation with HarshiCorp vault](https://www.hashicorp.com/resources/painless-password-rotation-hashicorp-vault)

3. Random password generation:
  - AWS secrets manager does have a `GetRandomPassword` API for generating random passwords. Though the same may not be supported for other secret services, and the generated passwords may not be consistent across different secret services.

4. Native rotation support in AWS Secrets Manager, is not used:
  - Minimum auto rotation period is 1 day, which is way too slow for what's desired.
  - `RotateSecret` requires a lambda function for non RDS/DocumentDB/Redshift secrets.
