mock_provider "random" {
  mock_resource "random_id" {
    defaults = { hex = "abcd0123" }
  }
}

mock_provider "http" {
  mock_data "http" {
    defaults = {
      response_body = "{\"cluster_name\":\"test-cluster\"}"
      status_code   = 200
    }
  }
}

mock_provider "azurerm" {
  mock_data "azurerm_client_config" {
    defaults = {
      subscription_id = "00000000-0000-0000-0000-000000000000"
      tenant_id       = "11111111-1111-1111-1111-111111111111"
      client_id       = "22222222-2222-2222-2222-222222222222"
      object_id       = "33333333-3333-3333-3333-333333333333"
    }
  }
  mock_resource "azurerm_user_assigned_identity" {
    defaults = {
      id           = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/test-rg/providers/Microsoft.ManagedIdentity/userAssignedIdentities/test-identity"
      principal_id = "44444444-4444-4444-4444-444444444444"
      client_id    = "55555555-5555-5555-5555-555555555555"
      tenant_id    = "11111111-1111-1111-1111-111111111111"
    }
  }
  mock_resource "azurerm_role_definition" {
    defaults = {
      role_definition_resource_id = "/subscriptions/00000000-0000-0000-0000-000000000000/providers/Microsoft.Authorization/roleDefinitions/fake-role-id"
    }
  }
  mock_resource "azurerm_role_assignment" {}
  mock_resource "azurerm_federated_identity_credential" {}
}

mock_provider "teleport" {
  mock_resource "teleport_integration" {}
  mock_resource "teleport_discovery_config" {}
  mock_resource "teleport_provision_token" {}
}

variables {
  teleport_proxy_public_addr      = "example.teleport.sh:443"
  teleport_discovery_group_name   = "test-group"
  azure_resource_group_name       = "test-rg"
  azure_managed_identity_location = "eastus"
  azure_matchers = [{
    types         = ["vm"]
    subscriptions = ["00000000-0000-0000-0000-000000000000"]
  }]
}

run "create" {
  assert {
    condition     = length(azurerm_user_assigned_identity.teleport_discovery_service) == 1
    error_message = "azurerm_user_assigned_identity should be created"
  }
  assert {
    condition     = length(azurerm_federated_identity_credential.teleport_discovery_service) == 1
    error_message = "azurerm_federated_identity_credential should be created"
  }
  assert {
    condition     = length(azurerm_role_definition.teleport_discovery) == 1
    error_message = "azurerm_role_definition should be created"
  }
  assert {
    condition     = length(teleport_integration.azure_oidc) == 1
    error_message = "teleport_integration should be created"
  }
  assert {
    condition     = length(teleport_discovery_config.azure) == 1
    error_message = "teleport_discovery_config should be created"
  }
  assert {
    condition     = length(teleport_provision_token.azure) == 1
    error_message = "teleport_provision_token should be created"
  }
}

run "use_oidc_integration_false" {
  variables {
    use_oidc_integration = false
  }

  assert {
    condition     = length(azurerm_user_assigned_identity.teleport_discovery_service) == 1
    error_message = "azurerm_user_assigned_identity should be created"
  }
  assert {
    condition     = length(azurerm_federated_identity_credential.teleport_discovery_service) == 0
    error_message = "azurerm_federated_identity_credential should not be created"
  }
  assert {
    condition     = length(teleport_integration.azure_oidc) == 0
    error_message = "teleport_integration should not be created"
  }
  assert {
    condition     = length(teleport_discovery_config.azure) == 1
    error_message = "teleport_discovery_config should be created"
  }
  assert {
    condition     = try(teleport_discovery_config.azure[0].spec.azure[0].integration, null) == null
    error_message = "discovery config should not reference an integration"
  }
}

run "create_azure_managed_identity_false" {
  variables {
    use_oidc_integration            = false
    create_azure_managed_identity   = false
    azure_resource_group_name       = null
    azure_managed_identity_location = null
  }

  assert {
    condition     = length(azurerm_user_assigned_identity.teleport_discovery_service) == 0
    error_message = "azurerm_user_assigned_identity should not be created"
  }
  assert {
    condition     = length(azurerm_role_definition.teleport_discovery) == 0
    error_message = "azurerm_role_definition should not be created"
  }
  assert {
    condition     = length(teleport_integration.azure_oidc) == 0
    error_message = "teleport_integration should not be created"
  }
  assert {
    condition     = length(teleport_discovery_config.azure) == 1
    error_message = "teleport_discovery_config should be created"
  }
  assert {
    condition     = length(teleport_provision_token.azure) == 1
    error_message = "teleport_provision_token should be created"
  }
}

run "teleport_integration_requires_azure_managed_identity" {
  command = plan

  variables {
    use_oidc_integration          = true
    create_azure_managed_identity = false
  }

  expect_failures = [teleport_integration.azure_oidc[0]]
}

run "azure_resource_group_name_required" {
  command = plan

  variables {
    azure_resource_group_name = null
  }

  expect_failures = [azurerm_user_assigned_identity.teleport_discovery_service[0]]
}

run "azure_managed_identity_location_required" {
  command = plan

  variables {
    azure_managed_identity_location = null
  }

  expect_failures = [azurerm_user_assigned_identity.teleport_discovery_service[0]]
}

run "discovery_config_subscription_wildcard_missing_scopes_error" {
  command = plan

  variables {
    azure_matchers = [{
      types         = ["vm"]
      subscriptions = ["*"]
    }]
    teleport_provision_token_allow_rules = [{
      subscription = "00000000-0000-0000-0000-000000000000"
    }]
  }

  expect_failures = [teleport_discovery_config.azure[0]]
}

run "discovery_config_subscription_wildcard_missing_allow_rules_error" {
  command = plan

  variables {
    azure_matchers = [{
      types         = ["vm"]
      subscriptions = ["*"]
    }]
    azure_role_assignment_scopes         = ["/providers/Microsoft.Management/managementGroups/my-mg"]
    teleport_provision_token_allow_rules = null
  }

  expect_failures = [teleport_provision_token.azure[0]]
}

run "discovery_config_subscription_wildcard" {
  variables {
    azure_matchers = [{
      types         = ["vm"]
      subscriptions = ["*"]
    }]
    azure_role_assignment_scopes = ["/providers/Microsoft.Management/managementGroups/my-mg"]
    teleport_provision_token_allow_rules = [{
      subscription = "00000000-0000-0000-0000-000000000000"
    }]
  }

  assert {
    condition     = length(teleport_discovery_config.azure) == 1
    error_message = "missing discovery config for wildcard"
  }
  assert {
    condition     = length(teleport_provision_token.azure) == 1
    error_message = "missing provision token for wildcard"
  }
}

run "discovery_config_resource_group_allow_rules" {
  variables {
    azure_matchers = [{
      types           = ["vm"]
      subscriptions   = ["aaaaaaaa-0000-0000-0000-000000000000", "bbbbbbbb-0000-0000-0000-000000000000"]
      resource_groups = ["my-rg"]
    }]
  }

  assert {
    condition     = length(teleport_provision_token.azure[0].spec.azure.allow) == 2
    error_message = "teleport_provision_token should have one allow rule per subscription"
  }
  assert {
    condition     = teleport_provision_token.azure[0].spec.azure.allow[0].subscription == "aaaaaaaa-0000-0000-0000-000000000000"
    error_message = "teleport_provision_token allow rule[0] should contain the first subscription"
  }
  assert {
    condition     = contains(teleport_provision_token.azure[0].spec.azure.allow[0].resource_groups, "my-rg")
    error_message = "teleport_provision_token allow rule[0] should contain the resource group"
  }
  assert {
    condition     = teleport_provision_token.azure[0].spec.azure.allow[1].subscription == "bbbbbbbb-0000-0000-0000-000000000000"
    error_message = "teleport_provision_token allow rule[1] should contain the second subscription"
  }
  assert {
    condition     = contains(teleport_provision_token.azure[0].spec.azure.allow[1].resource_groups, "my-rg")
    error_message = "teleport_provision_token allow rule[1] should contain the resource group"
  }
}

run "discovery_config_with_one_subscription" {
  assert {
    condition     = length(azurerm_role_assignment.teleport_discovery) == 1
    error_message = "azurerm_role_assignment does not match subscription count"
  }
  assert {
    condition     = contains(keys(azurerm_role_assignment.teleport_discovery), "/subscriptions/00000000-0000-0000-0000-000000000000")
    error_message = "azurerm_role_assignment missing subscription scope"
  }
}

run "discovery_config_with_multiple_subscriptions" {
  variables {
    azure_matchers = [{
      types         = ["vm"]
      subscriptions = ["aaaaaaaa-0000-0000-0000-000000000000", "bbbbbbbb-0000-0000-0000-000000000000"]
    }]
  }

  assert {
    condition     = length(azurerm_role_assignment.teleport_discovery) == 2
    error_message = "azurerm_role_assignment does not match subscription count"
  }
  assert {
    condition     = contains(keys(azurerm_role_assignment.teleport_discovery), "/subscriptions/aaaaaaaa-0000-0000-0000-000000000000")
    error_message = "azurerm_role_assignment missing subscription scope"
  }
  assert {
    condition     = contains(keys(azurerm_role_assignment.teleport_discovery), "/subscriptions/bbbbbbbb-0000-0000-0000-000000000000")
    error_message = "azurerm_role_assignment missing subscription scope"
  }
}

run "discovery_config_with_duplicate_subscriptions" {
  variables {
    azure_matchers = [
      { types = ["vm"], subscriptions = ["aaaaaaaa-0000-0000-0000-000000000000"] },
      { types = ["vm"], subscriptions = ["aaaaaaaa-0000-0000-0000-000000000000"] },
    ]
  }

  assert {
    condition     = length(azurerm_role_assignment.teleport_discovery) == 1
    error_message = "azurerm_role_assignment should have one assignment per subscription"
  }
}

run "azure_role_explicit_scopes_overrides_matchers" {
  variables {
    azure_matchers = [
      { types = ["vm"], subscriptions = ["aaaaaaaa-0000-0000-0000-000000000000", "aaaaaaaa-0000-0000-0000-000000000001"] },

    ]

    azure_role_assignment_scopes = ["/providers/Microsoft.Management/managementGroups/my-mg"]
  }

  assert {
    condition     = length(azurerm_role_assignment.teleport_discovery) == 1
    error_message = "azure_role_assignment_scopes does not match azurerm_role_assignment count"
  }

  assert {
    condition     = contains(keys(azurerm_role_assignment.teleport_discovery), "/providers/Microsoft.Management/managementGroups/my-mg")
    error_message = "azurerm_role_assignment does not include management group"
  }
}

run "name_suffix_applied_by_default" {
  assert {
    condition     = endswith(azurerm_user_assigned_identity.teleport_discovery_service[0].name, "-abcd0123")
    error_message = "azurerm_user_assigned_identity does not contain suffix"
  }
  assert {
    condition     = endswith(azurerm_role_definition.teleport_discovery[0].name, "-abcd0123")
    error_message = "azurerm_role_definition name does not contain suffix"
  }
  assert {
    condition     = endswith(teleport_integration.azure_oidc[0].metadata.name, "-abcd0123")
    error_message = "teleport_integration name does not contain suffix"
  }
  assert {
    condition     = endswith(teleport_discovery_config.azure[0].header.metadata.name, "-abcd0123")
    error_message = "teleport_discovery_config name does not contain suffix"
  }
  assert {
    condition     = endswith(teleport_provision_token.azure[0].metadata.name, "-abcd0123")
    error_message = "teleport_provision_token name does not contain suffix"
  }
}

run "name_suffix_exact" {
  variables {
    azure_managed_identity_name               = "my-identity"
    azure_managed_identity_use_name_prefix    = false
    azure_role_definition_name                = "my-role"
    azure_role_definition_use_name_prefix     = false
    teleport_integration_name                 = "my-integration"
    teleport_integration_use_name_prefix      = false
    teleport_discovery_config_name            = "my-discovery"
    teleport_discovery_config_use_name_prefix = false
    teleport_provision_token_name             = "my-token"
    teleport_provision_token_use_name_prefix  = false
  }

  assert {
    condition     = azurerm_user_assigned_identity.teleport_discovery_service[0].name == "my-identity"
    error_message = "azurerm_user_assigned_identity name is not exact"
  }
  assert {
    condition     = azurerm_role_definition.teleport_discovery[0].name == "my-role"
    error_message = "azurerm_role_definition name is not exact"
  }
  assert {
    condition     = teleport_integration.azure_oidc[0].metadata.name == "my-integration"
    error_message = "teleport_integration name is not exact"
  }
  assert {
    condition     = teleport_discovery_config.azure[0].header.metadata.name == "my-discovery"
    error_message = "teleport_discovery_config is not exact"
  }
  assert {
    condition     = teleport_provision_token.azure[0].metadata.name == "my-token"
    error_message = "teleport_provision_token name is not exact"
  }
}
