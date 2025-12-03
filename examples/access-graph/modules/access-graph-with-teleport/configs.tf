# Configuration Generation Modules
#
# This file handles all YAML configuration generation for Teleport, Access
# Graph, and Docker Compose files.

# Access Graph config for local deployment
module "access_graph_config_local" {
  source = "../config/access-graph-config"
}

# Teleport config for local deployment
module "teleport_config_local" {
  count  = var.local_deployment != null ? 1 : 0
  source = "../config/teleport-config"

  nodename            = var.name
  address             = local.local_teleport_address
  enable_access_graph = true
  enable_ssh_service  = var.teleport != null ? var.teleport.enable_ssh_service : false
}

# Docker compose for local deployment
module "docker_compose_local" {
  count  = var.local_deployment != null ? 1 : 0
  source = "../config/docker-compose"

  teleport_image     = var.teleport.image
  teleport_hostname  = local.local_teleport_address
  access_graph_image = var.access_graph.image
  enable_ssh_service = var.teleport.enable_ssh_service
}
