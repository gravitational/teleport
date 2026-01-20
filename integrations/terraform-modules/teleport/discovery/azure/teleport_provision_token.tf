################################################################################
# Teleport cluster provision token for Azure
################################################################################

# Teleport provision token for Azure join
resource "teleport_provision_token" "azure" {
  count = local.create ? 1 : 0

  metadata = {
    description = "Allow Teleport nodes to join the cluster using Azure credentials."
    labels      = local.apply_teleport_resource_labels
    name = (
      var.teleport_provision_token_use_name_prefix
      ? "${var.teleport_provision_token_name}-${local.teleport_resource_name_suffix}"
      : var.teleport_provision_token_name
    )
  }
  spec = {
    azure = {
      allow = [{
        subscription = local.azure_subscription_id
      }]
    }
    join_method = "azure"
    roles       = ["Node"]
  }
  version = "v2"
}
