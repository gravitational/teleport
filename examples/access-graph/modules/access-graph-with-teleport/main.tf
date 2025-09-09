# Access Graph with Teleport Module
# This module creates and configures infrastructure for Teleport and Access
# Graph deployments.

locals {
  local_teleport_address = coalesce(try(var.teleport.address, null), "${var.name}.local")
}

# Local Deployment
module "local" {
  count      = var.local_deployment != null ? 1 : 0
  source     = "../deployment/local"
  depends_on = [module.mkcert, module.internal_certs_local]

  target_dir          = var.local_deployment.target_dir
  docker_compose_yaml = module.docker_compose_local[0].yaml_content

  # Teleport
  teleport_config_yaml    = module.teleport_config_local[0].yaml_content
  license_pem_source_path = var.teleport.license_pem_path

  # Access Graph
  access_graph_config_yaml    = module.access_graph_config_local.yaml_content
  access_graph_ca_source_path = local.access_graph_ca_source
}
