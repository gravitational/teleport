resource "teleport_provision_token" "agent" {
  count = var.create && var.kubernetes_service.enabled ? 1 : 0

  version = "v2"
  metadata = {
    name        = "${local.container_group_name}-kubernetes"
    description = "Join the ACI Kubernetes Service using its managed identity."
  }
  spec = {
    roles       = ["Kube"]
    join_method = "generic_oidc"
    generic_oidc = {
      issuer   = "https://sts.windows.net/${data.azurerm_client_config.this[0].tenant_id}/"
      audience = local.azure_join_audience
      allow_any = [{
        conditions = [
          { attribute = "tid", eq = { value = data.azurerm_client_config.this[0].tenant_id } },
          { attribute = "oid", eq = { value = local.managed_identity.principal_id } },
          { attribute = "appid", eq = { value = local.managed_identity.client_id } },
        ]
      }]
    }
  }
}

locals {
  azure_join_audience = "https://management.azure.com/"

  teleport_join_params = var.create && var.kubernetes_service.enabled ? {
    token_name = nonsensitive(teleport_provision_token.agent[0].metadata.name)
    method     = "generic_oidc"
    generic_oidc = {
      http_request = {
        request = {
          url = "http://169.254.169.254/metadata/identity/oauth2/token"
          headers = {
            Metadata = "true"
          }
          query_params = {
            "api-version" = "2018-02-01"
            resource      = local.azure_join_audience
            client_id     = local.managed_identity.client_id
          }
        }
        result = {
          json_path = "$.access_token"
        }
      }
    }
  } : null
}
