data "azuread_application_published_app_ids" "well_known" {}

data "azuread_service_principal" "graph_api" {
  client_id = data.azuread_application_published_app_ids.well_known.result["MicrosoftGraph"]
}

data "azuread_client_config" "current" {}

