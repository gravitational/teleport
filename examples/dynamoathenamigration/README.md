# Dynamomigration tool

Dynamomigration tool allows to export Teleport audit events logs from DynamoDB
table into Athena Audit log.
It's using DynamoDB export to S3 to export data.

Requirements:

* Point-in-time recovery (PITR) on DynamoDB table
* Writable filesystem on machine where script will be executed
* IAM permissions:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "AllowDynamoExportAndList",
            "Effect": "Allow",
            "Action": [
                "dynamodb:ExportTableToPointInTime"
            ],
            "Resource": "arn:aws:dynamodb:region:account:table/tablename"
        },
        {
            "Sid": "AllowDynamoExportDescribe",
            "Effect": "Allow",
            "Action": [
                "dynamodb:DescribeExport"
            ],
            "Resource": "arn:aws:dynamodb:region:account:table/tablename/*"
        },
        {
            "Sid": "AllowWriteReadDestinationBucket",
            "Effect": "Allow",
            "Action": [
                "s3:AbortMultipartUpload",
                "s3:PutObject",
                "s3:PutObjectAcl",
                "s3:GetObject"
            ],
            "Resource": "arn:aws:s3:::export-bucket/*"
        },
        {
            "Sid": "AllowWriteLargePayloadsBucket",
            "Effect": "Allow",
            "Action": [
                "s3:AbortMultipartUpload",
                "s3:PutObject",
                "s3:PutObjectAcl"
            ],
            "Resource": "arn:aws:s3:::large-payloads-bucket/*"
        },
        {
            "Sid": "AllowPublishToAthenaTopic",
            "Effect": "Allow",
            "Action": [
                "sns:Publish"
            ],
            "Resource": "arn:aws:sns:region:account:topicname"
        }
    ]
}
```

## Example usage

Build: `cd examples/dynamoathenamigration/cmd && go build -o dynamoathenamigration`.

It is recommended to test export first using `-dryRun` flag. DryRun does not emit any events,
it makes sure that export is in valid format and events can be parsed.

Dry run example:

```shell
./dynamoathenamigration -dynamoARN='arn:aws:dynamodb:region:account:table/tablename' \
  -exportPath='s3://bucket/prefix' \
  -dryRun
```

Full migration:

```shell
./dynamoathenamigration -dynamoARN='arn:aws:dynamodb:region:account:table/tablename' \
  -exportPath='s3://bucket/prefix' \
  -snsTopicARN=arn:aws:sns:region:account:topicname \
  -largePayloadsPath=s3://bucket/prefix
```

To reuse existing export without triggering new one, use `-exportARN=xxx`.
