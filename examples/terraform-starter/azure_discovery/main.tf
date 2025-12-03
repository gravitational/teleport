# Will use az cli authentication automatically

data "azuread_client_config" "current" {}

# Resource Group for Teleport Infrastructure (Identity, etc.)
# This group will contain the User Assigned Identity used by the Teleport Discovery Service.
resource "azurerm_resource_group" "infra" {
  name     = "${var.prefix}-infra-rg"
  location = var.region
  tags     = var.tags
}

# -------------------------------------------------------------------------------------------------
# Managed Identity (Option 1)
# -------------------------------------------------------------------------------------------------

# User Assigned Identity for Teleport Discovery
# This identity is used by the Teleport Discovery Service to authenticate with Azure
# and scan for resources (VMs) to enroll.
resource "azurerm_user_assigned_identity" "teleport" {
  count               = var.use_managed_identity ? 1 : 0
  location            = var.region
  name                = "${var.prefix}-discovery-identity"
  resource_group_name = azurerm_resource_group.infra.name
  tags                = var.tags
}

# Federated Identity Credential for Managed Identity
# Trusts the Teleport Proxy as an OIDC issuer.
resource "azurerm_federated_identity_credential" "teleport" {
  count               = var.use_managed_identity ? 1 : 0
  name                = "${var.prefix}-teleport-federation"
  resource_group_name = azurerm_resource_group.infra.name
  parent_id           = azurerm_user_assigned_identity.teleport[0].id
  audience            = ["api://AzureADTokenExchange"]
  issuer              = "https://${replace(var.proxy_addr, ":443", "")}"
  subject             = "teleport-azure"
}

# -------------------------------------------------------------------------------------------------
# Service Principal (Option 2)
# -------------------------------------------------------------------------------------------------

resource "azuread_application" "teleport" {
  count            = var.use_managed_identity ? 0 : 1
  display_name     = "${var.prefix}-discovery-app"
  sign_in_audience = "AzureADMyOrg"
  owners           = [data.azuread_client_config.current.object_id]
}

resource "azuread_service_principal" "teleport" {
  count     = var.use_managed_identity ? 0 : 1
  client_id = azuread_application.teleport[0].client_id
  owners    = [data.azuread_client_config.current.object_id]
}

# Federated Identity Credential for Service Principal
resource "azuread_application_federated_identity_credential" "teleport" {
  count          = var.use_managed_identity ? 0 : 1
  application_id = "/applications/${azuread_application.teleport[0].object_id}"
  display_name   = "${var.prefix}-teleport-federation"
  audiences      = ["api://AzureADTokenExchange"]
  issuer         = "https://${replace(var.proxy_addr, ":443", "")}"
  subject        = "teleport-azure"
}

# -------------------------------------------------------------------------------------------------
# Common Resources
# -------------------------------------------------------------------------------------------------

locals {
  # Determine which Client ID and Principal ID to use based on the selected method
  client_id    = var.use_managed_identity ? azurerm_user_assigned_identity.teleport[0].client_id : azuread_application.teleport[0].client_id
  principal_id = var.use_managed_identity ? azurerm_user_assigned_identity.teleport[0].principal_id : azuread_service_principal.teleport[0].object_id
}

# Teleport Provision Token
# This token is used by the Teleport Discovery Service to join the Teleport cluster.
resource "teleport_provision_token" "azure_token" {
  version = "v2"

  metadata = {
    name = "${var.prefix}-azure-token"
  }

  spec = {
    roles       = ["Node"]
    join_method = "azure"
    azure = {
      allow = [{
        subscription = var.subscription_id
      }]
    }
  }
}

# Teleport Integration
# Creates an Azure OIDC integration in Teleport.
resource "teleport_integration" "azure_oidc" {
  version  = "v1"
  sub_kind = "azure-oidc"
  metadata = {
    name = "${var.prefix}-azure-oidc"
  }
  spec = {
    azure_oidc = {
      client_id = local.client_id
      tenant_id = var.tenant_id
    }
  }
}

# Teleport Discovery Configuration
# Configures the Teleport Discovery Service to look for Azure VMs.
resource "teleport_discovery_config" "azure_teleport" {
  header = {
    version = "v1"
    metadata = {
      name = "${var.prefix}-azure_teleport"
    }
  }

  spec = {
    discovery_group = "${var.discovery_group_name}"
    azure = [{
      types           = ["vm"]
      regions         = [var.region]
      subscriptions   = [var.subscription_id]
      resource_groups = [var.discovery_resource_group_name]
      integration     = teleport_integration.azure_oidc.metadata.name

      tags = {
        "*" = ["*"]
      }

      install_params = {
        join_method = "azure"
        join_token  = teleport_provision_token.azure_token.metadata.name
        script_name = "default-installer"
        azure       = {}
      }

      # install_params = {
      #   proxy_addr  = var.proxy_addr
      #   join_method = "azure"
      #   script_name = "default-installer"
      #   join_token  = var.token_name
      #   azure = {
      #     client_id = var.node_managed_identity_client_id
      #   }
      # }
    }]
  }
}

# Custom Role Definition for Teleport Discovery
resource "azurerm_role_definition" "teleport_discovery" {
  name  = "${var.prefix}-discovery-role"
  scope = "/subscriptions/${var.subscription_id}"

  permissions {
    actions = [
      "Microsoft.Compute/virtualMachines/read",
      "Microsoft.Compute/virtualMachines/runCommand/action",
      "Microsoft.Compute/virtualMachines/runCommands/write",
      "Microsoft.Compute/virtualMachines/runCommands/read",
      "Microsoft.Compute/virtualMachines/runCommands/delete",
    ]

    not_actions = []
  }

  assignable_scopes = [
    "/subscriptions/${var.subscription_id}",
  ]
}

# Role Assignment for Teleport Discovery
resource "azurerm_role_assignment" "teleport_discovery_assignment" {
  scope              = "/subscriptions/${var.subscription_id}"
  role_definition_id = azurerm_role_definition.teleport_discovery.role_definition_resource_id
  principal_id       = local.principal_id
}
