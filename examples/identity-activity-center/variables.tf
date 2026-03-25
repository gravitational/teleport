variable "aws_region" {
  description = "AWS region"
  default     = "us-west-2"
}

variable "iac_sqs_queue_name" {
  description = "Name of the SQS queue used for Identity Activity Center."
}

variable "iac_sqs_dlq_name" {
  description = "Name of the SQS Dead-Letter Queue used for handling unprocessable events"
}

variable "max_receive_count" {
  description = "Number of times a message can be received before it is sent to the DLQ"
  default     = 20
}

variable "iac_kms_key_alias" {
  description = "The alias of a custom KMS key"
}

variable "iac_long_term_bucket_name" {
  description = "Name of the long term storage bucket used for storing audit logs"
}

variable "iac_transient_bucket_name" {
  description = "Name of the transient storage bucket used for storing query results and large events payloads"
}

variable "iac_database_name" {
  description = "Name of Glue database"
}

variable "iac_table_name" {
  description = "Name of Glue table"
}

variable "iac_workgroup" {
  description = "Name of Athena iac_workgroup"
}

variable "iac_workgroup_max_scanned_bytes_per_query" {
  description = "Limit per query of max scanned bytes"
  default     = 21474836480 # 20GB
}

