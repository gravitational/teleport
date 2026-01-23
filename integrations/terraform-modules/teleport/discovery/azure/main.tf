terraform {
  required_version = ">= 1.6.0"

  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.0"
    }
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "~> 18.7"
    }
  }
}

locals {
  subscription_id           = var.subscription_id
  tenant_id                 = var.tenant_id
  region                    = var.region
  discovery_resource_groups = var.discovery_resource_group_names
  proxy_addr                = var.proxy_addr
  discovery_group           = var.discovery_group_name
  # Use the specified identity resource group, or default to the first discovery resource group
  # if none is provided. This allows managed identity groups to be in a separate resource group
  # from where VMs are discovered.
  identity_resource_group = coalesce(
    var.identity_resource_group_name,
    local.discovery_resource_groups[0]
  )
  tags = var.tags

  names = {
    identity         = "${var.prefix}-discovery-identity"
    federation       = "${var.prefix}-teleport-federation"
    token            = var.token_name_override != "" ? var.token_name_override : "${var.prefix}-azure-token"
    integration      = var.integration_name_override != "" ? var.integration_name_override : "${var.prefix}-azure-oidc"
    discovery_config = "${var.prefix}-azure-teleport"
    role             = "${var.prefix}-discovery-role"
  }

  # Extract the host from proxy_addr (format: host:port) to construct the OIDC issuer URL
  issuer = "https://${split(":", local.proxy_addr)[0]}"
}

# User-assigned managed identity for discovery
resource "azurerm_user_assigned_identity" "teleport" {
  location            = local.region
  name                = local.names.identity
  resource_group_name = local.identity_resource_group
  tags                = local.tags
}

# Federated identity credential for the managed identity (trust Teleport proxy issuer)
resource "azurerm_federated_identity_credential" "teleport" {
  name                = local.names.federation
  resource_group_name = local.identity_resource_group
  parent_id           = azurerm_user_assigned_identity.teleport.id
  audience            = ["api://AzureADTokenExchange"]
  issuer              = local.issuer
  subject             = "teleport-azure"
}



# Client and principal IDs from the managed identity
locals {
  client_id    = azurerm_user_assigned_identity.teleport.client_id
  principal_id = azurerm_user_assigned_identity.teleport.principal_id
}

# Teleport provision token for Azure join
resource "teleport_provision_token" "azure_token" {
  version = "v2"

  metadata = {
    name = local.names.token
    labels = {
      "teleport.dev/origin" = "terraform"
    }
  }

  spec = {
    roles       = ["Node"]
    join_method = "azure"
    azure = {
      allow = [{
        subscription = local.subscription_id
      }]
    }
  }
}

# Teleport Azure OIDC integration using the selected identity
resource "teleport_integration" "azure_oidc" {
  version  = "v1"
  sub_kind = "azure-oidc"
  metadata = {
    name = local.names.integration
    labels = {
      "teleport.dev/origin" = "terraform"
    }
  }
  spec = {
    azure_oidc = {
      client_id = local.client_id
      tenant_id = local.tenant_id
    }
  }
}

# Teleport discovery config targeting Azure VMs
resource "teleport_discovery_config" "azure_teleport" {
  header = {
    version = "v1"
    metadata = {
      name = local.names.discovery_config
      labels = {
        "teleport.dev/origin" = "terraform"
      }
    }
  }

  spec = {
    discovery_group = local.discovery_group
    azure = [{
      types           = ["vm"]
      regions         = [local.region]
      subscriptions   = [local.subscription_id]
      resource_groups = local.discovery_resource_groups
      integration     = teleport_integration.azure_oidc.metadata.name

      tags = var.discovery_tags

      install_params = {
        join_method = "azure"
        join_token  = teleport_provision_token.azure_token.metadata.name
        script_name = var.installer_script_name
      }
    }]
  }
}

# Custom role for discovery permissions
resource "azurerm_role_definition" "teleport_discovery" {
  name  = local.names.role
  scope = "/subscriptions/${local.subscription_id}"

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
    "/subscriptions/${local.subscription_id}",
  ]
}

# Assign the custom role to the managed identity principal
resource "azurerm_role_assignment" "teleport_discovery_assignment" {
  scope              = "/subscriptions/${local.subscription_id}"
  role_definition_id = azurerm_role_definition.teleport_discovery.role_definition_resource_id
  principal_id       = local.principal_id
}

