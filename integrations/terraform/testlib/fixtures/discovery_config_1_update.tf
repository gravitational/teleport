resource "teleport_discovery_config" "test" {
  header = {
    metadata = {
      name        = "test"
      description = "Updated example azure discovery config"
      labels = {
        foo = "baz"
      }
    }
    version = "v1"
  }
  spec = {
    discovery_group = "azure_teleport_updated"
    azure = [{
      types           = ["vm", "aks"]
      regions         = ["westus", "eastus"]
      subscriptions   = ["456456-456456-456456-456456"]
      resource_groups = ["group", "group2"]

      tags = {
        "env" = ["prod"]
      }

      install_params = {
        join_method = "azure"
        script_name = "updated-installer"
        join_token  = "azure-token-updated"
        azure = {
          client_id = "managed-identity-id-updated"
        }
      }
    }]
  }
}


