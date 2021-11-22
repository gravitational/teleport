## DynamoDB backend implementation for Teleport.

### Introduction

This package enables Teleport auth server to store secrets in 
[DynamoDB](https://aws.amazon.com/dynamodb/) on AWS.

WARNING: Using DynamoDB involves recurring charge from AWS.

The table created by the backend will provision 5/5 R/W capacity.
It should be covered by the free tier.

### Running tests

The DynamodDB tests are not run by default. To run them locally, try:

```
go test -tags dynamodb -v  ./lib/backend/dynamo
```

*NOTE:* you will need to provide a AWS credentials & a default region 
(e.g. in your `~/.aws/credentials` & `~/.aws/config` files, or via
environment vars) for the tests to work.

### Quick Start

Add this storage configuration in `teleport` section of the config file (by default it's `/etc/teleport.yaml`):

```yaml
teleport:
  storage:
    type: dynamodb
    region: eu-west-1
    table_name: teleport.state
    access_key: XXXXXXXXXXXXXXXXXXXXX
    secret_key: YYYYYYYYYYYYYYYYYYYYY
```

Replace `region` and `table_name` with your own settings. Teleport will create the table automatically.

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
