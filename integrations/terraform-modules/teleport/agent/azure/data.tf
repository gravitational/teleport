data "azurerm_client_config" "this" {
  count = var.create ? 1 : 0
}

# Look up the current Azure Management Group by ID.
# Used to retrieve all the current subscriptions under that management group.
data "azurerm_management_group" "agent" {
  count = local.create_managed_identity_role && local.managed_identity_role_scope_override == null && var.azure_management_group_id != null ? 1 : 0

  name = var.azure_management_group_id
}

data "http" "managed_updates" {
  count = var.create && var.managed_updates_enabled ? 1 : 0

  url = format(
    "https://%v/webapi/find?group=%v",
    local.managed_updates_proxy_addr,
    urlencode(coalesce(var.managed_updates_group, "default")),
  )

  lifecycle {
    precondition {
      condition     = !var.managed_updates_enabled || local.managed_updates_proxy_addr != ""
      error_message = "Managed Updates require teleport.proxy_server in teleport_config."
    }

    postcondition {
      condition = self.status_code == 200 && can(
        regex(
          "^v?[0-9]+\\.[0-9]+\\.[0-9]+.*",
          trimspace(jsondecode(self.response_body).auto_update.agent_version),
        )
      )
      error_message = <<EOF
Managed Updates endpoint must return HTTP 200 and a valid Teleport version.
Ensure the cluster supports Managed Updates and the configured update group exists.
EOF
    }
  }
}
