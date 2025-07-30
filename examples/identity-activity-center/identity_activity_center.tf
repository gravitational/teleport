# ========================================
# Identity Activity Center Infrastructure
# ========================================
#
# This Terraform configuration provisions a comprehensive Identity Activity Center 
# infrastructure on AWS designed to collect, store, and analyze identity-related 
# events from various sources including AWS services, GitHub, Okta, and Teleport.
#
# Architecture Overview:
# 1. Event Collection: Events received via SQS queue
# 2. Processing: Events processed and stored in S3 buckets  
# 3. Storage: Long-term storage in partitioned Parquet format
# 4. Analysis: Querying via Amazon Athena with Glue Data Catalog
#
# Security Features:
# - All data encrypted at rest using customer-managed KMS keys
# - S3 buckets with public access blocked
# - Versioning enabled for data protection
# - Dead letter queue for reliable message processing
#
# Data Partitioning Strategy:
# - tenant_id: Enables multi-tenant data separation
# - event_date: Daily partitions for efficient querying (4-year range)

# ========================================
# Provider Configuration
# ========================================
provider "aws" {
  region = var.aws_region
}

# Get current AWS account information.
data "aws_caller_identity" "current" {}

# ========================================
# Encryption Infrastructure
# ========================================

# Customer-managed KMS key for encrypting all Identity Activity Center data
# This key is used across S3 buckets, SQS queues, and Athena query results
resource "aws_kms_key" "identity_activity_center_encryption_key" {
  description         = "KMS key for Athena audit log"
  enable_key_rotation = true
}

# KMS key policy granting full access to the current AWS account
# In production, consider restricting to specific roles/users
resource "aws_kms_key_policy" "identity_activity_center_encryption_key_policy" {
  key_id = aws_kms_key.identity_activity_center_encryption_key.id
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

# Human-readable alias for the KMS key
resource "aws_kms_alias" "identity_activity_center_encryption_key_alias" {
  name          = "alias/${var.iac_kms_key_alias}"
  target_key_id = aws_kms_key.identity_activity_center_encryption_key.key_id
}


# ========================================
# Message Queue Infrastructure
# ========================================

# Dead Letter Queue for messages that fail processing
# Retains messages for 7 days
# This provides time for troubleshooting failed message processing

resource "aws_sqs_queue" "identity_activity_center_queue_dlq" {
  name = var.iac_sqs_dlq_name

  # Encrypt messages using the Identity Activity Center KMS key
  kms_master_key_id                 = aws_kms_key.identity_activity_center_encryption_key.arn
  kms_data_key_reuse_period_seconds = 300 # 5 minutes - balances security and performance

  # Extended retention for troubleshooting failed messages
  message_retention_seconds = 604800 # 7 days

  tags = {
    Name      = "Identity Activity Center DLQ"
    Purpose   = "Dead letter queue for failed identity event processing"
    Component = "MessageQueue"
  }
}


# Main SQS queue for receiving identity events from various sources
# Events include authentication attempts, access requests, and administrative actions
resource "aws_sqs_queue" "identity_activity_center_queue" {
  name = var.iac_sqs_queue_name

  # Encrypt all messages using customer-managed KMS key
  kms_master_key_id                 = aws_kms_key.identity_activity_center_encryption_key.arn
  kms_data_key_reuse_period_seconds = 300

  # Configure dead letter queue for failed message processing
  # Messages are moved to DLQ after max_receive_count failed processing attempts
  redrive_policy = jsonencode({
    deadLetterTargetArn = aws_sqs_queue.identity_activity_center_queue_dlq.arn
    maxReceiveCount     = var.max_receive_count
  })

  tags = {
    Name      = "Identity Activity Center Main Queue"
    Purpose   = "Primary queue for identity event ingestion"
    Component = "MessageQueue"
  }
}



# ========================================
# Long-term Storage Infrastructure
# ========================================

# S3 bucket for permanent storage of processed identity events
# Data is stored in Parquet format with daily partitions by tenant_id and event_date
resource "aws_s3_bucket" "identity_activity_center_long_term_storage" {
  bucket = var.iac_long_term_bucket_name

  # Allow destruction for development - disable in production
  force_destroy = true

  # Object lock provides immutable storage for compliance requirements
  # Recommended for production environments to prevent accidental deletion
  object_lock_enabled = false

  tags = {
    Name      = "Identity Activity Center Long-term Storage"
    Purpose   = "Permanent storage for processed identity events"
    Component = "Storage"
    DataType  = "LongTerm"
  }
}


# Enable KMS encryption for all objects in the long-term storage bucket
resource "aws_s3_bucket_server_side_encryption_configuration" "identity_activity_center_long_term_storage" {
  bucket = aws_s3_bucket.identity_activity_center_long_term_storage.id

  rule {
    apply_server_side_encryption_by_default {
      kms_master_key_id = aws_kms_key.identity_activity_center_encryption_key.arn
      sse_algorithm     = "aws:kms"
    }
    # S3 Bucket Keys reduce KMS API calls and costs
    bucket_key_enabled = true
  }
}

# Enforce bucket owner control over all objects
resource "aws_s3_bucket_ownership_controls" "identity_activity_center_long_term_storage" {
  bucket = aws_s3_bucket.identity_activity_center_long_term_storage.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}
# Enable versioning for data protection and compliance
resource "aws_s3_bucket_versioning" "identity_activity_center_long_term_storage" {
  bucket = aws_s3_bucket.identity_activity_center_long_term_storage.id

  versioning_configuration {
    status = "Enabled"
  }
}


# Block all public access to protect sensitive identity data
resource "aws_s3_bucket_public_access_block" "identity_activity_center_long_term_storage" {
  bucket = aws_s3_bucket.identity_activity_center_long_term_storage.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}



# ========================================
# Transient Storage Infrastructure  
# ========================================

# S3 bucket for temporary data storage including:
# - Athena query results
# - Processing intermediate files
# - Large files during processing
resource "aws_s3_bucket" "identity_activity_center_transient_storage" {
  bucket = var.iac_transient_bucket_name

  force_destroy = true

  tags = {
    Name      = "Identity Activity Center Transient Storage"
    Purpose   = "Temporary storage for processing and query results"
    Component = "Storage"
    DataType  = "Transient"
  }
}


# S3 bucket deletes the files after 60 days.
resource "aws_s3_bucket_lifecycle_configuration" "identity_activity_center_transient_bucket_lifecycle_config" {
  bucket = aws_s3_bucket.identity_activity_center_transient_storage.id

  rule {
    status = "Enabled"
    id     = "delete_after_60_days"
    filter {}

    expiration {
      days = 60
    }
  }
}

# Apply same encryption configuration as long-term storage
resource "aws_s3_bucket_server_side_encryption_configuration" "identity_activity_center_transient_storage" {
  bucket = aws_s3_bucket.identity_activity_center_transient_storage.id

  rule {
    apply_server_side_encryption_by_default {
      kms_master_key_id = aws_kms_key.identity_activity_center_encryption_key.arn
      sse_algorithm     = "aws:kms"
    }
    bucket_key_enabled = true
  }
}

resource "aws_s3_bucket_ownership_controls" "identity_activity_center_transient_storage" {
  bucket = aws_s3_bucket.identity_activity_center_transient_storage.id

  rule {
    object_ownership = "BucketOwnerEnforced"
  }
}

resource "aws_s3_bucket_versioning" "identity_activity_center_transient_storage" {
  bucket = aws_s3_bucket.identity_activity_center_transient_storage.id

  versioning_configuration {
    status = "Enabled"
  }
}

resource "aws_s3_bucket_public_access_block" "identity_activity_center_transient_storage" {
  bucket = aws_s3_bucket.identity_activity_center_transient_storage.id

  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}


# ========================================
# Data Catalog Infrastructure
# ========================================

# Glue database serves as a logical container for table metadata
# Used by Athena for querying identity event data
resource "aws_glue_catalog_database" "identity_activity_center_db" {
  name = var.iac_database_name

  description = "Database containing identity activity event tables for multi-tenant analytics"
}

# Glue table definition for identity events with comprehensive schema
# Supports data from multiple identity providers: AWS, GitHub, Okta, Teleport
resource "aws_glue_catalog_table" "identity_activity_center_table" {
  name          = var.iac_table_name
  database_name = aws_glue_catalog_database.identity_activity_center_db.name
  table_type    = "EXTERNAL_TABLE"

  description = "Identity activity events table with partition projection for efficient querying"

  # Table parameters configure partition projection and storage format
  parameters = {
    "EXTERNAL"            = "TRUE"
    "classification"      = "parquet"
    "parquet.compression" = "SNAPPY"

    # Partition projection automatically generates partition metadata
    # Eliminates need for manual partition management
    "projection.enabled" = "true"

    # tenant_id projection - injected values for multi-tenant support
    "projection.tenant_id.type" = "injected"

    # event_date projection - daily partitions with 4-year range
    "projection.event_date.type"          = "date"
    "projection.event_date.format"        = "yyyy-MM-dd"
    "projection.event_date.interval"      = "1"
    "projection.event_date.interval.unit" = "DAYS"
    "projection.event_date.range"         = "NOW-4YEARS,NOW"

    # S3 location template for partitioned data
    "storage.location.template" = format("s3://%s/data/$${tenant_id}/$${event_date}/", aws_s3_bucket.identity_activity_center_long_term_storage.bucket)
  }

  # Storage descriptor defines data format and location
  storage_descriptor {
    location      = format("s3://%s/data/", aws_s3_bucket.identity_activity_center_long_term_storage.bucket)
    input_format  = "org.apache.hadoop.hive.ql.io.parquet.MapredParquetInputFormat"
    output_format = "org.apache.hadoop.hive.ql.io.parquet.MapredParquetOutputFormat"

    ser_de_info {
      name                  = "identity-events-parquet-serde"
      serialization_library = "org.apache.hadoop.hive.ql.io.parquet.serde.ParquetHiveSerDe"
      parameters = {
        "serialization.format" = "1"
      }
    }

    # Comprehensive schema for identity events from multiple sources
    # Core identity and authentication fields
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

# ========================================
# Analytics Infrastructure
# ========================================

# Athena workgroup for managing query execution and cost controls
# Provides isolated environment for identity analytics queries
resource "aws_athena_workgroup" "identity_activity_center_workgroup" {
  name = var.iac_workgroup

  # Allow destruction for development environments
  force_destroy = true

  description = "Workgroup for Identity Activity Center analytics with cost controls and encryption"

  configuration {
    # Cost control - limit maximum data scanned per query
    # Adjust based on expected query patterns and cost requirements
    bytes_scanned_cutoff_per_query = var.iac_workgroup_max_scanned_bytes_per_query

    # Use latest Athena engine for best performance and features
    engine_version {
      selected_engine_version = "Athena engine version 3"
    }

    # Configure encrypted storage for query results
    result_configuration {
      output_location = format("s3://%s/results/", aws_s3_bucket.identity_activity_center_transient_storage.bucket)

      encryption_configuration {
        encryption_option = "SSE_KMS"
        kms_key_arn       = aws_kms_key.identity_activity_center_encryption_key.arn
      }
    }

  }

  tags = {
    Name      = "Identity Activity Center Workgroup"
    Purpose   = "Analytics workgroup for identity event queries"
    Component = "Analytics"
  }
}

# ========================================
# Configuration Output
# ========================================

# Generate YAML configuration for applications consuming this infrastructure
# Provides all necessary connection details and endpoints
output "identity_activity_center_yaml" {
  description = "YAML configuration for Identity Activity Center applications"

  value = <<EOT
# Identity Activity Center Configuration
# Generated by Terraform - contains all necessary connection details
identity_activity_center:
  # AWS Region where resources are deployed
  region: ${var.aws_region}
  
  # Athena database and table for querying identity events
  database: ${var.iac_database_name}
  table: '${var.iac_table_name}'
  
  # S3 storage locations
  s3: '${format("s3://%s/data", aws_s3_bucket.identity_activity_center_long_term_storage.bucket)}'
  s3_results: '${format("s3://%s/results", aws_s3_bucket.identity_activity_center_transient_storage.bucket)}'
  s3_large_files: '${format("s3://%s/large_files", aws_s3_bucket.identity_activity_center_transient_storage.bucket)}'
  
  # SQS queue for event ingestion
  sqs_queue_url: '${aws_sqs_queue.identity_activity_center_queue.url}'
  
  # Athena workgroup for query execution
  workgroup: '${aws_athena_workgroup.identity_activity_center_workgroup.name}'
  
  # MaxMind GeoIP database path (configure based on your setup)
  maxmind_geoip_city_db_path: './geo...'
  
EOT
}

# ========================================
# Additional Outputs for Integration
# ========================================

output "kms_key_arn" {
  description = "ARN of the KMS key used for encryption"
  value       = aws_kms_key.identity_activity_center_encryption_key.arn
  sensitive   = false
}

output "sqs_queue_url" {
  description = "URL of the main SQS queue for event ingestion"
  value       = aws_sqs_queue.identity_activity_center_queue.url
}

output "sqs_dlq_url" {
  description = "URL of the dead letter queue"
  value       = aws_sqs_queue.identity_activity_center_queue_dlq.url
}

output "long_term_bucket_name" {
  description = "Name of the S3 bucket for long-term storage"
  value       = aws_s3_bucket.identity_activity_center_long_term_storage.bucket
}

output "transient_bucket_name" {
  description = "Name of the S3 bucket for transient storage"
  value       = aws_s3_bucket.identity_activity_center_transient_storage.bucket
}

output "database_name" {
  description = "Name of the Glue database"
  value       = aws_glue_catalog_database.identity_activity_center_db.name
}

output "table_name" {
  description = "Name of the Glue table"
  value       = aws_glue_catalog_table.identity_activity_center_table.name
}

output "athena_workgroup_name" {
  description = "Name of the Athena workgroup"
  value       = aws_athena_workgroup.identity_activity_center_workgroup.name
}