variable "target_dir" {
  description = "Target directory where to write output files"
  type        = string
  default     = "./out"
}

variable "access_graph_config_yaml" {
  description = "Access Graph configuration YAML content to write"
  type        = string
  default     = ""
}

variable "geoip_db_source_path" {
  description = "Source path of the GeoIP database file to copy"
  type        = string
  default     = ""
}

variable "license_pem_source_path" {
  description = "Source path of the license PEM file to copy"
  type        = string
  default     = ""
}

variable "teleport_config_yaml" {
  description = "Teleport configuration YAML content to write"
  type        = string
  default     = ""
}

variable "docker_compose_yaml" {
  description = "Docker Compose YAML content to write"
  type        = string
  default     = ""
}

variable "access_graph_ca_source_path" {
  description = "Source path of the Access Graph CA certificate file to copy to teleport/certs"
  type        = string
  default     = ""
}
