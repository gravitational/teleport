variable "aws_region" {
  description = "AWS region"
  default     = "us-west-2"
}

variable "sqs_queue_name" {
  description = "Name of the SQS queue used for Identity Activity Center."
}

variable "sqs_dlq_name" {
  description = "Name of the SQS Dead-Letter Queue used for handling unprocessable events"
}

variable "max_receive_count" {
  description = "Number of times a message can be received before it is sent to the DLQ"
  default     = 20
}

variable "kms_key_alias" {
  description = "The alias of a custom KMS key"
}

variable "long_term_bucket_name" {
  description = "Name of the long term storage bucket used for storing audit logs"
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

variable "workgroup_max_scanned_bytes_per_query" {
  description = "Limit per query of max scanned bytes"
  default     = 21474836480 # 20GB
}

# search_event_limiter variables allows to configured rate limit on top of
# search events API to prevent increasing costs in case of aggressive use of API.
# In current version Athena Audit logger is not prepared for polling of API.
# Burst=20, time=1m and amount=5, means that you can do 20 requests without any
# throttling, next requests will be throttled, and tokens will be filled to
# rate limit bucket at amount 5 every 1m.
variable "search_event_limiter_burst" {
  description = "Number of tokens available for rate limit used on top of search event API"
  default     = 20
}

variable "search_event_limiter_time" {
  description = "Duration between the addition of tokens to the bucket for rate limit used on top of search event API"
  default     = "1m"
}

variable "search_event_limiter_amount" {
  description = "Number of tokens added to the bucket during specific interval for rate limit used on top of search event API"
  default     = 5
}
