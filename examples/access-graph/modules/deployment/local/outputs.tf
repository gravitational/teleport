output "output_dir" {
  description = "The target directory path"
  value       = var.target_dir
}

output "access_graph_config_path" {
  description = "Path to the Access Graph config file"
  value       = var.access_graph_config_yaml != "" ? local_file.access_graph_config[0].filename : ""
}

output "geoip_database_path" {
  description = "Path to the GeoIP database file"
  value       = var.geoip_db_source_path != "" ? local_file.geoip_database[0].filename : ""
}

output "license_pem_path" {
  description = "Path to the license PEM file"
  value       = var.license_pem_source_path != "" ? local_file.license_pem[0].filename : ""
}

output "teleport_config_path" {
  description = "Path to the Teleport config file"
  value       = var.teleport_config_yaml != "" ? local_file.teleport_config[0].filename : ""
}

output "docker_compose_path" {
  description = "Path to the Docker Compose file"
  value       = var.docker_compose_yaml != "" ? local_file.docker_compose[0].filename : ""
}
