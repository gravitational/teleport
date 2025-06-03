data "aws_partition" "current" {}

data "aws_region" "current" {}

data "aws_iam_policy_document" "identity_activity_center_policy" {
  statement {
    sid = "AllowListingMultipartUploads"

    actions = [
      "s3:ListBucketMultipartUploads",
      "s3:GetBucketLocation",
      "s3:ListBucketVersions",
      "s3:ListBucket"
    ]

    resources = [
      aws_s3_bucket.identity_activity_center_transient_storage.arn,
      aws_s3_bucket.identity_activity_center_long_term_storage.arn,
    ]
  }

  statement {
    sid = "AllowMultipartAndObjectAccess"
    actions = [
      "s3:PutObject",
      "s3:ListMultipartUploadParts",
      "s3:GetObjectVersion",
      "s3:GetObject",
      "s3:DeleteObjectVersion",
      "s3:DeleteObject",
      "s3:AbortMultipartUpload"
    ]

    resources = [
      format("%s/data/*", aws_s3_bucket.identity_activity_center_long_term_storage.arn),
      format("%s/results/*", aws_s3_bucket.identity_activity_center_transient_storage.arn),
      format("%s/large_files/*", aws_s3_bucket.identity_activity_center_transient_storage.arn),
    ]

  }

  statement {
    sid = "AllowPublishReceiveSQS"
    actions = [
      "sqs:ReceiveMessage",
      "sqs:DeleteMessage",
      "sqs:SendMessage"
    ]

    resources = [
      aws_sqs_queue.identity_activity_center_queue.arn
    ]

  }

  statement {
    sid = "AllowAthenaQuery"
    actions = [
      "glue:GetTable",
      "athena:StartQueryExecution",
      "athena:GetQueryResults",
      "athena:GetQueryExecution"
    ]

    resources = [
      aws_glue_catalog_table.identity_activity_center_table.arn,
      aws_glue_catalog_database.identity_activity_center_db.arn,
      aws_glue_catalog_table.identity_activity_center_table.arn,
      aws_athena_workgroup.identity_activity_center_workgroup.arn,
      "arn:${data.aws_partition.current.partition}:glue:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:catalog",
    ]

  }

  statement {
    sid = "AllowAthenaKMSUsage"
    actions = [
      "kms:GenerateDataKey",
      "kms:Decrypt"
    ]

    resources = [
      aws_kms_key.identity_activity_center_encryption_key.arn,
    ]

  }

}


output "identity_activity_center_iam_policy" {
  value = data.aws_iam_policy_document.identity_activity_center_policy.json
}
