locals {
  resource_matcher_labels = var.teleport_discovery_group_name != "" ? merge(
    var.kubernetes_service.resource_matcher_labels, {
      "teleport.internal/discovery-group-name" = [var.teleport_discovery_group_name]
  }) : var.kubernetes_service.resource_matcher_labels

  teleport_config = {
    version = "v3"
    teleport = {
      proxy_server = trimspace(var.teleport_proxy_public_addr)
      join_params  = local.teleport_join_params
      log = {
        output = "stderr"
        format = { output = "json" }
      }
    }
    auth_service      = { enabled = false }
    proxy_service     = { enabled = false }
    ssh_service       = { enabled = false }
    app_service       = { enabled = false }
    db_service        = { enabled = false }
    discovery_service = { enabled = false }
    kubernetes_service = {
      enabled   = var.kubernetes_service.enabled
      resources = [{ labels = local.resource_matcher_labels }]
    }
  }
}
