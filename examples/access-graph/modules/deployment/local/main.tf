# Local Deployment Module
# This module handles all local file outputs

# Write Access Graph configuration file
resource "local_file" "access_graph_config" {
  count = var.access_graph_config_yaml != "" ? 1 : 0

  filename        = "${var.target_dir}/access-graph/config.yaml"
  content         = var.access_graph_config_yaml
  file_permission = "0644"
}

# Copy GeoIP database file
resource "local_file" "geoip_database" {
  count = var.geoip_db_source_path != "" ? 1 : 0

  filename        = "${var.target_dir}/access-graph/geolite2-city.mmdb"
  source          = var.geoip_db_source_path
  file_permission = "0644"
}

# Copy license PEM file to certs directory
resource "local_file" "license_pem" {
  count = var.license_pem_source_path != "" ? 1 : 0

  filename        = "${var.target_dir}/teleport/certs/license.pem"
  source          = var.license_pem_source_path
  file_permission = "0644"
}

# Write Teleport configuration file
resource "local_file" "teleport_config" {
  count = var.teleport_config_yaml != "" ? 1 : 0

  filename        = "${var.target_dir}/teleport/config.yaml"
  content         = var.teleport_config_yaml
  file_permission = "0644"
}

# Write Docker Compose configuration file
resource "local_file" "docker_compose" {
  count = var.docker_compose_yaml != "" ? 1 : 0

  filename        = "${var.target_dir}/docker-compose.yaml"
  content         = var.docker_compose_yaml
  file_permission = "0644"
}

# Note: mkcert certificates are generated directly to teleport/certs directory

# Note: Internal certificates are generated directly to access-graph/certs directory

# Copy Access Graph CA to Teleport certs directory for secure connection
resource "local_file" "teleport_access_graph_ca" {
  count = var.access_graph_ca_source_path != "" ? 1 : 0

  filename        = "${var.target_dir}/teleport/certs/internal-ca.crt"
  source          = var.access_graph_ca_source_path
  file_permission = "0644"
}
