variable "teleport_image" {
  description = "Docker image for Teleport"
  type        = string
  default     = "public.ecr.aws/gravitational/teleport-ent-distroless-debug:18.2.0"
}

variable "teleport_hostname" {
  description = "Hostname for Teleport service"
  type        = string
  default     = "tele.local"
}

variable "access_graph_image" {
  description = "Docker image for Access Graph"
  type        = string
  default     = "public.ecr.aws/gravitational/access-graph:1.29.0"
}

variable "postgres_image" {
  description = "Docker image for PostgreSQL"
  type        = string
  default     = "postgres:16"
}

variable "enable_ssh_service" {
  description = "Enable SSH service in Teleport container (adds post_start commands)"
  type        = bool
  default     = false
}
