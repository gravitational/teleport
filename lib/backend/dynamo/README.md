## DynamoDB backend implementation for Teleport.

### Introduction

This package enables Teleport auth server to store secrets in 
[DynamoDB](https://aws.amazon.com/dynamodb/) on AWS.

WARNING: Using DynamoDB involves recurring charge from AWS.

### Running tests

The DynamoDB tests are not run by default. To run them locally, try:

```
TELEPORT_DYNAMODB_TEST=true go test -v  ./lib/backend/dynamo
```

*NOTE:* you will need to provide a AWS credentials and a default region
(e.g. in your `~/.aws/credentials` & `~/.aws/config` files, or via
environment vars) for the tests to work.

Here's one way to achieve that:

```
echo '{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "AWS": "*"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}' > assume-policy.json
aws iam create-role --role-name dynamodb-tests-role --assume-role-policy-document file://assume-policy.json

echo '{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "dynamodb:*"
      ],
      "Effect": "Allow",
      "Resource": "*"
    }
  ]
}' > dynamodb-access.json
aws iam put-role-policy --role-name dynamodb-tests-role --policy-name dynamodb-access --policy-document file://dynamodb-access.json

aws sts assume-role --role-arn $(aws iam get-role --role-name dynamodb-tests-role | jq '.Role.Arn' | xargs) --role-session-name session-test > credentials.json

export AWS_ACCESS_KEY_ID=$(cat credentials.json | jq '.Credentials.AccessKeyId' | xargs)
export AWS_SECRET_ACCESS_KEY=$(cat credentials.json | jq '.Credentials.SecretAccessKey' | xargs)
export AWS_SESSION_TOKEN=$(cat credentials.json | jq '.Credentials.SessionToken' | xargs)
```
