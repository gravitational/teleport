resource "teleport_integration" "azure_oidc" {
  version  = "v1"
  sub_kind = "azure-oidc"
  metadata = {
    name        = "azure-oidc"
    description = "Test integration"
    labels = {
      example = "yes"
    }
  }

  spec = {
    azure_oidc = {
      tenant_id = "updated-tenant"
      client_id = "updated-client"
    }
  }
}
