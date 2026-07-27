variable "teleport_version" {
  default     = "19.0.0-prealpha.2"
  description = <<EOD
The version of Teleport to deploy.
Generally, the version of Teleport should be controlled by using the appropriate version of this module.
This variable is intended for development usage.
EOD
  type        = string
}
