# Teleport Usage Gathering Script

This script retrieves the number of unique users accessing each of the five
Teleport supported protocols over a 30 day period.

## Prerequisites

This tool requires a Teleport cluster running with AWS DynamoDB as your backend
server. This script is intended to run as a docker container from either the
auth server or a server with IAM permissions necessary to run queries on the
DynamoDB events table.

> **_NOTE:_** Minimum IAM permission can be accomplished by assigning AWS IAM
> policy `AmazonDynamoDBReadOnlyAccess`
The following information is required:

| Environment Variable | Description                                                         |
| ---------------------|---------------------------------------------------------------------|
| `TABLE_NAME`         | DynamoDB Events Table Name                                          |
| `AWS_REGION`         | AWS Region where the dynamoDB table is deployed                     |
| `START_DATE`         | The date for when to start the query. The format must be YYYY-MM-DD |

Optionally, the environment variable `SHOW_USERS` can be set to `true` to display a list of users for each protocol.

## Running Docker Container

With prompt:

```console
$ docker run -it --rm public.ecr.aws/gravitational/teleport-usage:<VERSION>
```

With environment variables:

```console
$ docker run -it --rm public.ecr.aws/gravitational/teleport-usage:<VERSION> \
-e "TABLE_NAME=cluster-events" -e "AWS_REGION=us-east-1" \
-e "START_DATE=2022-12-01"
```
