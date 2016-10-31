## DynamoDB backend implementation for Teleport.

### Introduction

This package enables Teleport auth server to store secrets in 
[DynamoDB](https://aws.amazon.com/dynamodb/) on AWS.

WARNING: Using DynamoDB involves reccuring charge from AWS.

The table created by the backend will provision 5/5 R/W capacity.
It should be covered by the free tier.

### Building

DynamoDB backend is not enabled by default. To enable it you have to 
compile Teleport with `dynamo` build flag.

To build Teleport with DynamoDB enabled, run:

```
ADDFLAGS='-tags dynamodb' make teleport
```

### Quick Start

Install the auth server and add this storage configuration in 
Teleport section of the config file (by default it's `/etc/teleport.yaml`):

```
teleport:
  storage:
    type: dynamodb
    region: eu-west-1
    table_name: prod.teleport.auth
    access_key: XXXXXXXXXXXXXXXXXXXXX
    secret_key: YYYYYYYYYYYYYYYYYYYYY
```

Replace `region` and `table_name` with the appropriate settings.

### AWS IAM Role

You can use IAM role instead of hard coded access and secret key (IAM role is
recommended).  You must apply correct policy in order to the auth to
create/get/update K/V in DynamoDB.

Example of a typical policy (change region and account ID):

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AllAPIActionsOnTeleportAuth",
            "Effect": "Allow",
            "Action": "dynamodb:*",
            "Resource": "arn:aws:dynamodb:eu-west-1:123456789012:table/prod.teleport.auth"
        }
    ]
}
```

### Get Help

This backend has been contributed by https://github.com/apestel
