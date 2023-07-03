# Teleport Usage Gathering Script
<a href="https://gallery.ecr.aws/gravitational/teleport-usage">
<img src="https://img.shields.io/github/v/release/gravitational/teleport?sort=semver&label=Container Image&color=621FFF" />
</a>


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

## Running Docker Container

This script is dependent of environment variables being set. Below is an example on how to run the script in Docker using environment variables:

> **_NOTE:_** The latest container image version can be found at the top of this page. This version is independent of your Teleport cluster.

```console
$ docker run -it --rm -e "TABLE_NAME=cluster-events" \
    -e "AWS_REGION=us-east-1" \
    -e "START_DATE=2022-12-01" \ 
    public.ecr.aws/gravitational/teleport-usage:<container-version>
```
