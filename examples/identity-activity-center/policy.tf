# ========================================
# Identity Activity Center IAM Policy
# ========================================
#
# This Terraform configuration creates comprehensive IAM policies for the Identity 
# Activity Center infrastructure. The policy grants least-privilege access to AWS 
# services required for identity event processing, storage, and analytics.
#
# Policy Coverage:
# - S3 bucket and object operations for data storage and retrieval
# - SQS queue operations for event message processing
# - Athena and Glue operations for data analytics and querying
# - KMS operations for encryption/decryption of data
#
# Usage:
# This policy should be attached to IAM roles used by:
# - Lambda functions processing identity events
# - EC2 instances running data processing workloads
# - ECS/Fargate tasks handling identity analytics
# - Applications querying identity data via Athena
#
# Security Principles:
# - Least privilege access with specific resource ARNs
# - Separate permissions for different operation types
# - Resource-specific access patterns (e.g., data/* for objects)
# - KMS permissions limited to necessary operations

# ========================================
# Data Sources for Dynamic ARN Construction
# ========================================

# Get current AWS partition (aws, aws-gov, or aws-cn)
# Used for constructing ARNs that work across different AWS partitions
data "aws_partition" "current" {}

# Get current AWS region
# Used for constructing region-specific ARNs like Glue catalog
data "aws_region" "current" {}

# ========================================
# Identity Activity Center IAM Policy Document
# ========================================

# Comprehensive IAM policy document providing least-privilege access
# to all AWS services required by the Identity Activity Center
data "aws_iam_policy_document" "identity_activity_center_policy" {

  # ========================================
  # S3 Bucket-Level Permissions
  # ========================================

  # Allow listing and metadata operations on S3 buckets
  # Required for multipart uploads, bucket discovery, and versioning operations
  statement {
    sid = "AllowListingMultipartUploads"

    effect = "Allow"

    actions = [
      "s3:ListBucketMultipartUploads", # List incomplete multipart uploads
      "s3:GetBucketLocation",          # Get bucket region for cross-region operations
      "s3:ListBucketVersions",         # List object versions (versioning enabled)
      "s3:ListBucket"                  # List objects in bucket
    ]

    # Apply to both storage buckets used by Identity Activity Center
    resources = [
      aws_s3_bucket.identity_activity_center_transient_storage.arn, # Temporary storage
      aws_s3_bucket.identity_activity_center_long_term_storage.arn, # Permanent storage
    ]
  }

  # ========================================
  # S3 Object-Level Permissions
  # ========================================

  # Allow object operations within specific prefixes
  # Supports large file uploads via multipart upload mechanism
  statement {
    sid = "AllowMultipartAndObjectAccess"

    effect = "Allow"

    actions = [
      "s3:PutObject",                # Upload new objects
      "s3:ListMultipartUploadParts", # List parts of multipart upload
      "s3:GetObjectVersion",         # Get specific version of object
      "s3:GetObject",                # Download objects
      "s3:DeleteObjectVersion",      # Delete specific object version
      "s3:DeleteObject",             # Delete objects
      "s3:AbortMultipartUpload"      # Cancel incomplete multipart uploads
    ]

    # Resource-specific access patterns for different data types
    resources = [
      # Long-term storage: processed identity events in Parquet format
      # Organized by tenant_id and date partitions
      format("%s/data/*", aws_s3_bucket.identity_activity_center_long_term_storage.arn),

      # Transient storage: Athena query results
      # Temporary files cleaned up by lifecycle policies
      format("%s/results/*", aws_s3_bucket.identity_activity_center_transient_storage.arn),

      # Transient storage: Large files during processing
      # Intermediate files that exceed memory limits
      format("%s/large_files/*", aws_s3_bucket.identity_activity_center_transient_storage.arn),
    ]
  }

  # ========================================
  # SQS Queue Permissions
  # ========================================

  # Allow message operations on the Identity Activity Center queue
  # Supports event ingestion workflow from various identity providers
  statement {
    sid = "AllowPublishReceiveSQS"

    effect = "Allow"

    actions = [
      "sqs:ReceiveMessage", # Receive identity events from queue
      "sqs:DeleteMessage",  # Remove processed messages
      "sqs:SendMessage"     # Send new identity events to queue
    ]

    # Apply only to the main Identity Activity Center queue
    # Dead letter queue access not included (separate permissions if needed)
    resources = [
      aws_sqs_queue.identity_activity_center_queue.arn
    ]
  }

  # ========================================
  # Athena and Glue Analytics Permissions
  # ========================================

  # Allow querying identity data via Athena and accessing Glue metadata
  # Supports analytics workloads and reporting capabilities
  statement {
    sid = "AllowAthenaQuery"

    effect = "Allow"

    actions = [
      # Glue Data Catalog operations
      "glue:GetTable", # Get table schema and metadata

      # Athena query operations
      "athena:StartQueryExecution", # Execute SQL queries
      "athena:GetQueryResults",     # Retrieve query results
      "athena:GetQueryExecution"    # Get query status and metadata
    ]

    resources = [
      # Glue resources for table and database metadata
      aws_glue_catalog_table.identity_activity_center_table.arn,
      aws_glue_catalog_database.identity_activity_center_db.arn,

      # Athena workgroup for query execution and cost control
      aws_athena_workgroup.identity_activity_center_workgroup.arn,

      # Glue Data Catalog (regional resource)
      # Required for Athena to access table metadata
      "arn:${data.aws_partition.current.partition}:glue:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:catalog",
    ]
  }

  # ========================================
  # KMS Encryption Permissions
  # ========================================

  # Allow encryption and decryption operations for Identity Activity Center data
  # Required for accessing encrypted S3 objects, SQS messages, and Athena results
  statement {
    sid = "AllowAthenaKMSUsage"

    effect = "Allow"

    actions = [
      "kms:GenerateDataKey", # Generate data keys for encryption
      "kms:Decrypt"          # Decrypt data keys and objects
    ]

    # Apply only to the Identity Activity Center KMS key
    # Prevents access to other KMS keys in the account
    resources = [
      aws_kms_key.identity_activity_center_encryption_key.arn,
    ]

  }
}

# ========================================
# Policy Output for External Use
# ========================================

# Output the complete IAM policy as JSON for use in IAM roles
# This policy can be attached to roles used by applications accessing
# the Identity Activity Center infrastructure
output "identity_activity_center_iam_policy" {
  description = "Complete IAM policy JSON for Identity Activity Center access"
  value       = data.aws_iam_policy_document.identity_activity_center_policy.json

}
