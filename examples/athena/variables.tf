variable "aws_region" {
  description = "AWS region"
  default     = "us-west-2"
}

variable "sns_topic_name" {
  description = "Name of the SNS topic used for publishing audit events"
}

variable "sqs_queue_name" {
  description = "Name of the SQS queue used for subscription for audit events topic"
}

variable "sqs_dlq_name" {
  description = "Name of the SQS Dead-Letter Queue used for handling unprocessable events"
}

variable "max_receive_count" {
  description = "Number of times a message can be received before it is sent to the DLQ"
  default     = 10
}

variable "kms_key_alias" {
  description = "The alias of a custom KMS key"
}

variable "long_term_bucket_name" {
  description = "Name of the long term storage bucket used for storing audit events"
}

variable "transient_bucket_name" {
  description = "Name of the transient storage bucket used for storing query results and large events payloads"
}

variable "database_name" {
  description = "Name of Glue database"
}

variable "table_name" {
  description = "Name of Glue table"
}

variable "workgroup" {
  description = "Name of Athena workgroup"
}
