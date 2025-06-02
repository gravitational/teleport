provider "aws" {
  region = var.aws_region
}

data "aws_caller_identity" "current" {}

resource "aws_kms_key" "encryption_key" {
  description         = "KMS key for Athena audit log"
  enable_key_rotation = true
}

resource "aws_kms_key_policy" "encryption_key_policy" {
  key_id = aws_kms_key.encryption_key.id
  policy = jsonencode({
    Statement = [
      {
        Action = [
          "kms:*"
        ]
        Effect = "Allow"
        Principal = {
          AWS = data.aws_caller_identity.current.account_id
        }
        Resource = "*"
        Sid      = "Default Policy"
      },
    ]
    Version = "2012-10-17"
  })
}

resource "aws_kms_alias" "encryption_key_alias" {
  name          = "alias/${var.kms_key_alias}"
  target_key_id = aws_kms_key.encryption_key.key_id
}

resource "aws_sqs_queue" "identity_activity_center_queue_dlq" {
  name                              = var.sqs_dlq_name
  kms_master_key_id                 = aws_kms_key.encryption_key.arn
  kms_data_key_reuse_period_seconds = 300
  message_retention_seconds         = 604800 // 7 days which is three days longer than default 4 of sqs queue
}

resource "aws_sqs_queue" "identity_activity_center_queue" {
  name                              = var.sqs_queue_name
  kms_master_key_id                 = aws_kms_key.encryption_key.arn
  kms_data_key_reuse_period_seconds = 300

  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.identity_activity_center_queue_dlq.arn
    maxReceiveCount     = var.max_receive_count
  })
}


resource "aws_s3_bucket" "long_term_storage" {
  bucket        = var.long_term_bucket_name
  force_destroy = true
  # On production we recommend enabling object lock to provide deletion protection.
  object_lock_enabled = false
}

resource "aws_s3_bucket_server_side_encryption_configuration" "long_term_storage" {
  bucket = aws_s3_bucket.long_term_storage.id
  rule {
    apply_server_side_encryption_by_default {
      kms_master_key_id = aws_kms_key.encryption_key.arn
      sse_algorithm     = "aws:kms"
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_ownership_controls" "long_term_storage" {
  bucket = aws_s3_bucket.long_term_storage.id
  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_s3_bucket_versioning" "long_term_storage" {
  bucket = aws_s3_bucket.long_term_storage.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_public_access_block" "long_term_storage" {
  bucket                  = aws_s3_bucket.long_term_storage.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_s3_bucket" "transient_storage" {
  bucket        = var.transient_bucket_name
  force_destroy = true
  # On production we recommend enabling lifecycle configuration to clean transient data.
}

resource "aws_s3_bucket_server_side_encryption_configuration" "transient_storage" {
  bucket = aws_s3_bucket.transient_storage.id
  rule {
    apply_server_side_encryption_by_default {
      kms_master_key_id = aws_kms_key.encryption_key.arn
      sse_algorithm     = "aws:kms"
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_ownership_controls" "transient_storage" {
  bucket = aws_s3_bucket.transient_storage.id
  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_s3_bucket_versioning" "transient_storage" {
  bucket = aws_s3_bucket.transient_storage.id
  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_public_access_block" "transient_storage" {
  bucket                  = aws_s3_bucket.transient_storage.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

resource "aws_glue_catalog_database" "identity_activity_center_db" {
  name = var.database_name
}

resource "aws_glue_catalog_table" "identity_activity_center_table" {
  name          = var.table_name
  database_name = aws_glue_catalog_database.identity_activity_center_db.name
  table_type    = "EXTERNAL_TABLE"
  parameters = {
    "EXTERNAL"                            = "TRUE",
    "projection.enabled"                  = "true",
    "projection.tenant_id.type"           = "injected",
    "projection.event_date.type"          = "date",
    "projection.event_date.format"        = "yyyy-MM-dd",
    "projection.event_date.interval"      = "1",
    "projection.event_date.interval.unit" = "DAYS",
    "projection.event_date.range"         = "NOW-4YEARS,NOW",
    "storage.location.template"           = format("s3://%s/data/$${tenant_id}/$${event_date}/", aws_s3_bucket.long_term_storage.bucket)
    "classification"                      = "parquet"
    "parquet.compression"                 = "SNAPPY",
  }
  storage_descriptor {
    location      = format("s3://%s", aws_s3_bucket.long_term_storage.bucket)
    input_format  = "org.apache.hadoop.hive.ql.io.parquet.MapredParquetInputFormat"
    output_format = "org.apache.hadoop.hive.ql.io.parquet.MapredParquetOutputFormat"
    ser_de_info {
      name                  = "example"
      parameters            = { "serialization.format" = "1" }
      serialization_library = "org.apache.hadoop.hive.ql.io.parquet.serde.ParquetHiveSerDe"
    }
    columns {
      name = "event_source"
      type = "string"
    }

    columns {
      name = "identity"
      type = "string"

    }

    columns {
      name = "identity_kind"
      type = "string"
    }

    columns {
      name = "identity_id"
      type = "string"
    }
    columns {
      name = "token"
      type = "string"

    }
    columns {
      name = "action"
      type = "string"

    }
    columns {
      name = "origin"
      type = "string"

    }
    columns {
      name = "status"
      type = "string"

    }
    columns {
      name = "ip"
      type = "string"

    }
    columns {
      name = "city"
      type = "string"

    }
    columns {
      name = "country"
      type = "string"

    }
    columns {
      name = "region"
      type = "string"

    }
    columns {
      name = "latitude"
      type = "double"

    }
    columns {
      name = "longitude"
      type = "double"

    }
    columns {
      name = "target_resource"
      type = "string"

    }
    columns {
      name = "target_kind"
      type = "string"

    }
    columns {
      name = "target_location"
      type = "string"

    }
    columns {
      name = "target_id"
      type = "string"

    }
    columns {
      name = "user_agent"
      type = "string"

    }
    columns {
      name = "event_type"
      type = "string"

    }
    columns {
      name = "event_time"
      type = "timestamp"

    }
    columns {
      name = "uid"
      type = "string"

    }
    columns {
      name = "event_data"
      type = "string"

    }
    columns {
      name = "aws_account_id"
      type = "string"

    }
    columns {
      name = "aws_service"
      type = "string"

    }
    columns {
      name = "github_organization"
      type = "string"

    }
    columns {
      name = "github_repo"
      type = "string"

    }
    columns {
      name = "okta_org"
      type = "string"

    }
    columns {
      name = "teleport_cluster"
      type = "string"

    }

  }
  partition_keys {
    name = "tenant_id"
    type = "string"
  }
  partition_keys {
    name = "event_date"
    type = "date"
  }
}

resource "aws_athena_workgroup" "workgroup" {
  name          = var.workgroup
  force_destroy = true
  configuration {
    bytes_scanned_cutoff_per_query = var.workgroup_max_scanned_bytes_per_query
    engine_version {
      selected_engine_version = "Athena engine version 3"
    }
    result_configuration {
      output_location = format("s3://%s/results", aws_s3_bucket.transient_storage.bucket)
      encryption_configuration {
        encryption_option = "SSE_KMS"
        kms_key_arn       = aws_kms_key.encryption_key.arn
      }
    }
  }
}



output "identity_activity_center_yaml" {
  value = <<EOT
identity_activity_center:
  region: ${var.aws_region}
  database: ${var.database_name}
  table: '${var.table_name}'
  s3:  '${format("s3://%s/data", var.long_term_bucket_name)}'
  s3_results: '${format("s3://%s/results", var.transient_bucket_name)}'
  s3_large_files: '${format("s3://%s/large_files", var.transient_bucket_name)}'
  sqs_queue_url: '${aws_sqs_queue.identity_activity_center_queue.url}'
  maxmind_geoip_city_db_path: './geo...'
EOT
}
