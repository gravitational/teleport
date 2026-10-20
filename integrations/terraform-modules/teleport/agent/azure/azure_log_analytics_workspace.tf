resource "azurerm_log_analytics_workspace" "this" {
  count = var.create ? 1 : 0

  location = var.location
  name = (
    var.log_analytics_workspace_use_name_prefix
    ? format("%v-%v",
      var.log_analytics_workspace_name,
      one(random_string.name_suffix[*].result),
    )
    : var.log_analytics_workspace_name
  )
  resource_group_name = var.resource_group_name
  retention_in_days   = var.log_analytics_workspace_retention_days
  tags                = var.apply_azure_tags
}
