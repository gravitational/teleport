locals {
  container_group_name = (
    var.create && var.container_group_use_name_prefix
    ? format("%v-%v",
      var.container_group_name,
      one(random_string.name_suffix[*].result),
    )
    : var.container_group_name
  )

  managed_updates_proxy_addr = try(
    replace(
      trimspace(var.teleport_proxy_public_addr),
      "/^[^:]*:[/]{2}/", # trim any URL scheme
    ""),
  "")
  managed_updates_version = (
    length(data.http.managed_updates) == 1
    ? jsondecode(data.http.managed_updates[0].response_body).auto_update.agent_version
    : null
  )
  teleport_version = trimprefix(
    trimspace(
      coalesce(
        var.teleport_version_override,
        local.managed_updates_version,
        local.module_version,
      ),
    ),
    "v"
  )
  teleport_environment = merge(
    var.environment_vars,
    {
      AZURE_CLIENT_ID                          = try(local.managed_identity.client_id, "")
      TELEPORT_INSTALL_METHOD_TERRAFORM_MODULE = "true"
    },
  )

  # Deployments are unique by name, so when creating a new deployment we need to ensure its name is unique.
  # The deployment_revision and create_before_destroy lifecycle rule allow two deployments to coexist temporarily during updates.
  deployment_revision = substr(sha256(jsonencode({
    config      = local.teleport_config
    environment = local.teleport_environment
    image       = "${var.teleport_container_image}:${local.teleport_version}"
    cpu         = var.container_group_cpu
    memory      = var.container_group_memory
    log_analytics = var.create ? {
      workspace_id    = azurerm_log_analytics_workspace.this[0].workspace_id
      key_fingerprint = nonsensitive(sha256(azurerm_log_analytics_workspace.this[0].primary_shared_key))
    } : null
    subnet_id = var.subnet_id
  })), 0, 8)
}

resource "azurerm_container_group" "teleport_agent" {
  count               = var.create && var.kubernetes_service.enabled ? var.container_group_desired_count : 0
  name                = "${local.container_group_name}-${local.deployment_revision}-${count.index}"
  location            = var.location
  resource_group_name = var.resource_group_name
  os_type             = "Linux"
  ip_address_type     = var.subnet_id == null ? "None" : "Private"
  subnet_ids          = var.subnet_id == null ? null : [var.subnet_id]
  restart_policy      = "Always"
  tags                = var.apply_azure_tags

  identity {
    type         = "UserAssigned"
    identity_ids = [local.managed_identity.id]
  }

  diagnostics {
    log_analytics {
      workspace_id  = azurerm_log_analytics_workspace.this[0].workspace_id
      workspace_key = azurerm_log_analytics_workspace.this[0].primary_shared_key
    }
  }

  container {
    name   = "teleport"
    image  = "${var.teleport_container_image}:${local.teleport_version}"
    cpu    = var.container_group_cpu
    memory = var.container_group_memory
    commands = [
      # rewrite SIGTERM (15) to SIGQUIT (3) so ACI stop signal triggers graceful Teleport shutdown
      "/usr/bin/dumb-init",
      "--rewrite",
      "15:3",
      "--",
      "teleport",
      "start",
      "--config-string",
      base64encode(yamlencode(local.teleport_config)),
    ]
    environment_variables = local.teleport_environment

    # ACI requires a port declaration when subnet_id is specified.
    # Teleport uses an outbound reverse tunnel and does not listen on this port.
    # The port number 49151 is the highest non-reserved port according to Azure docs:
    # https://learn.microsoft.com/en-us/azure/container-instances/container-instances-faq#does-the-aci-service-reserve-ports-for-service-functionality-
    dynamic "ports" {
      for_each = var.subnet_id == null ? [] : [49151]
      content {
        port     = ports.value
        protocol = "TCP"
      }
    }
  }

  lifecycle {
    precondition {
      condition     = !var.managed_updates_enabled || local.managed_updates_proxy_addr != ""
      error_message = "Managed Updates require teleport_proxy_public_addr."
    }

    # This ensures a new container is started before the old one is destroyed, reducing downtime.
    # This requires a unique name for the deployment which is generated in local.deployment_revision.
    create_before_destroy = true
  }

  # Identity permissions must be assigned before the agent starts.
  depends_on = [azurerm_role_assignment.teleport_agent]
}
