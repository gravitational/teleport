variable "teleport_version" {
  default     = "18.12.0-dev.alexh.1"
  description = <<EOD
The version of Teleport to deploy.
Generally, the version of Teleport should be controlled by using the appropriate version of this module.
This variable is intended for development usage.
EOD
  type        = string
}
