resource "aws_iam_role" "access_monitoring_role" {
  count = var.access_monitoring ? 1 : 0
  name  = "${var.access_monitoring_prefix}AccessMonitoringRole"
  assume_role_policy = jsonencode({
    "Version" : "2012-10-17",
    "Statement" : [
      {
        "Sid" : "IamPrincipal",
        "Effect" : "Allow",
        "Principal" : {
          "AWS" : [
            var.access_monitoring_trusted_relationship_role_arn != "" ? var.access_monitoring_trusted_relationship_role_arn : data.aws_caller_identity.current.arn
          ]
        },
        "Action" : [
          "sts:AssumeRole",
          "sts:TagSession"
        ]
      }
    ]
  })
}

resource "aws_iam_role_policy_attachment" "role_policy_attachment" {
  count      = var.access_monitoring ? 1 : 0
  role       = aws_iam_role.access_monitoring_role[0].name
  policy_arn = aws_iam_policy.access_monitoring_policy[0].arn
}

resource "aws_iam_policy" "access_monitoring_policy" {
  count  = var.access_monitoring ? 1 : 0
  name   = "${var.access_monitoring_prefix}AccessMonitoringPolicy"
  path   = "/"
  policy = data.aws_iam_policy_document.access_monitoring_policy[0].json
}

resource "aws_athena_workgroup" "access_monitoring_workgroup" {
  count         = var.access_monitoring ? 1 : 0
  name          = "${var.access_monitoring_prefix}access_monitoring_workgroup"
  force_destroy = true
  configuration {
    publish_cloudwatch_metrics_enabled = true
    bytes_scanned_cutoff_per_query     = 322122547200
    engine_version {
      selected_engine_version = "Athena engine version 3"
    }
    result_configuration {
      output_location = format("s3://%s/results", aws_s3_bucket.transient_storage.bucket)
      encryption_configuration {
        encryption_option = "SSE_KMS"
        kms_key_arn       = aws_kms_key.audit_key.arn
      }
    }
  }
  tags = {
    Name = "${var.access_monitoring_prefix}access_monitoring_workgroup"
  }
}

data "aws_iam_policy_document" "access_monitoring_policy" {
  count = var.access_monitoring ? 1 : 0
  statement {
    actions = [
      "s3:ListBucketMultipartUploads",
      "s3:GetBucketLocation",
      "s3:ListBucketVersions",
      "s3:ListBucket"
    ]
    resources = [
      aws_s3_bucket.transient_storage.arn,
      aws_s3_bucket.long_term_storage.arn,
    ]
  }
  statement {
    actions = [
      "s3:GetObject",
      "s3:GetObjectVersion",
      "s3:PutObject"
    ]
    resources = [
      "${aws_s3_bucket.long_term_storage.arn}/report_results/*",
      "${aws_s3_bucket.transient_storage.arn}/results/*"
    ]
  }

  statement {
    actions = [
      "s3:ListMultipartUploadParts",
      "s3:GetObjectVersion",
      "s3:GetObject",
      "s3:AbortMultipartUpload"
    ]
    resources = [
      "${aws_s3_bucket.transient_storage.arn}/results/*",
      "${aws_s3_bucket.long_term_storage.arn}/events/*",
      "${aws_s3_bucket.long_term_storage.arn}/report_results/*"
    ]
  }
  statement {
    actions = [
      "glue:GetTable",
      "athena:StartQueryExecution",
      "athena:GetQueryResults",
      "athena:GetQueryExecution"
    ]
    resources = [
      aws_glue_catalog_table.audit_table.arn,
      aws_glue_catalog_database.audit_db.arn,
      "arn:aws:glue:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:catalog",
      aws_athena_workgroup.access_monitoring_workgroup[0].arn,
    ]
  }
  statement {
    actions = [
      "kms:GenerateDataKey",
      "kms:Decrypt"
    ]
    resources = [
      aws_kms_key.audit_key.arn,
    ]
  }
}

data "aws_region" "current" {}

output "access_monitoring_configuration" {
  value = var.access_monitoring ? replace(yamlencode({
    "access_monitoring" : {
      enabled : true,
      role_arn : aws_iam_role.access_monitoring_role[0].arn,
      report_results : format("s3://%s/report_results", aws_s3_bucket.long_term_storage.bucket),
      workgroup : aws_athena_workgroup.access_monitoring_workgroup[0].name
    }
  }), "\"", "") : null
}

output "access_monitoring_chart_configuration" {
  value = var.access_monitoring ? replace(yamlencode({
    "aws" : {
      "accessMonitoring" : {
        enabled : true,
        roleARN : aws_iam_role.access_monitoring_role[0].arn,
        reportResults : format("s3://%s/report_results", aws_s3_bucket.long_term_storage.bucket),
        workgroup : aws_athena_workgroup.access_monitoring_workgroup[0].name
      }
    }
  }), "\"", "") : null
}
