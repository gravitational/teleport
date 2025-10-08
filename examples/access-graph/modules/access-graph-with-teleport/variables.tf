variable "name" {
  description = "Name and prefix for deployment"
  type        = string
}

variable "access_graph" {
  description = "Access Graph configuration"
  type = object({
    image = optional(string, "public.ecr.aws/gravitational/access-graph:1.28.1")
    identity_activity_center = optional(object({
      geoip_db_path = optional(string, "")
    }))
  })
}

variable "teleport" {
  description = "Teleport configuration"
  type = object({
    image              = optional(string, "public.ecr.aws/gravitational/teleport-ent-distroless-debug:18")
    license_pem_path   = string
    address            = optional(string, "")
    enable_ssh_service = optional(bool, true)
  })
}

variable "local_deployment" {
  description = "Local deployment configuration. If null, no local files will be created"
  type = object({
    target_dir = optional(string, "./out")
  })
  default = null
}
