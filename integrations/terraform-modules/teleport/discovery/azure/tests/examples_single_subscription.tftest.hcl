mock_provider "random" {
  mock_resource "random_id" {
    defaults = { hex = "deadbeef" }
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
      id           = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/example-resources/providers/Microsoft.ManagedIdentity/userAssignedIdentities/test-identity"
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
  mock_resource "azurerm_resource_group" {}
}

mock_provider "teleport" {
  mock_resource "teleport_integration" {}
  mock_resource "teleport_discovery_config" {}
  mock_resource "teleport_provision_token" {}
  mock_resource "teleport_installer" {}
}

run "validate" {
  command = apply
  module {
    source = "./examples/single-subscription"
  }
}
