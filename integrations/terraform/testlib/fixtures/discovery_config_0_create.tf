resource "teleport_discovery_config" "test" {
  header = {
    metadata = {
      name        = "test"
      description = "Example azure discovery config"
      labels = {
        foo = "bar"
      }
    }
    version = "v1"
  }
  spec = {
    discovery_group = "azure_teleport"
    azure = [{
      types           = ["vm"]
      regions         = ["eastus"]
      subscriptions   = ["123123-123123-123123-123123"]
      resource_groups = ["group"]

      tags = {
        "*" = ["*"]
      }

      install_params = {
        join_method = "azure"
        script_name = "default-installer"
        join_token  = "azure-token"
        azure = {
          client_id = "managed-identity-id"
        }
      }
    }]
  }
}
