variable "identifier" {
  description = "The name of the DocumentDB cluster"
  type        = string
}

// =====================================
// DocumentDB cluster related variables
// =====================================
variable "subnet_ids" {
  description = "A list of VPC subnet IDs"
  type        = list(string)
}

variable "security_group_ids" {
  description = "A list of VPC subnet IDs"
  type        = list(string)
}

variable "num_instances" {
  description = "Number of cluster instances"
  type        = number
  default     = 1
}

variable "instance_class" {
  description = "Instance class"
  type        = string
  default     = "db.t3.medium"
}

variable "tags" {
  description = "A mapping of tags to assign to all resources"
  type        = map(string)
  default     = {}
}

variable "engine_version" {
  description = "The engine version to use"
  type        = string
  default     = "5.0.0"

  validation {
    condition     = tonumber(split(".", var.engine_version)[0]) >= 5
    error_message = "IAM authentication is not supported for engine version below 5"
  }
}

variable "parameter_group_name" {
  description = "Parameter group family"
  type        = string
  // Note that default parameter group has TLS enabled.
  default = "default.docdb5.0"
}

variable "master_username" {
  description = "Master username"
  type        = string
  default     = "teleport"
}

variable "master_password" {
  description = "Master password"
  type        = string
  sensitive   = true
  default     = "MYsecretpassw0rd"
}

// ======================
// IAM related variables
// ======================
variable "create_discovery_iam_role" {
  description = "Creates an IAM role for discovery service."
  type        = bool
  default     = false
}

variable "create_access_iam_role" {
  description = "Creates an IAM role for database service."
  type        = bool
  default     = false
}

variable "create_database_user_iam_role" {
  description = "Creates an IAM role for database database service."
  type        = bool
  default     = false
}

variable "databaase_user_iam_role_trusted_role_arns" {
  description = "List of IAM roles trusted by the database user role. If create_access_iam_role is false, pass in the role ARN of the databse service."
  type        = list(string)
  default     = []
}

// ===========================
// Teleprot related variables
// ===========================
variable "create_teleport_databases" {
  description = "Creates dynamic Teleport databases."
  type        = bool
  default     = false
}

variable "create_teleport_databases_per_instance" {
  description = "When create_teleport_databases is true, creates one Teleport database per instance."
  type        = bool
  default     = true
}
