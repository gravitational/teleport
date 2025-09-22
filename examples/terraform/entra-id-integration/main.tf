# Configure Terraform
terraform {
  required_providers {
    azuread = {
      source  = "hashicorp/azuread"
      version = "~> 3.1.0"
    }
  }
}

provider "azuread" {
  tenant_id = var.tenant_id
}

locals {
  teleport_saml_entity_id = "https://${var.proxy_service_address}/v1/webapi/saml/acs/entra-id"
  owners                  = [data.azuread_client_config.current.object_id]
}

# Configure an enterprise application.
resource "azuread_application" "app" {
  display_name = var.app_name

  group_membership_claims = var.group_membership_claims

  web {
    # SAML Assertion Consumer Service URL.
    redirect_uris = [local.teleport_saml_entity_id]
  }

  owners = var.use_system_credentials ? concat(local.owners, [var.managed_id]) : local.owners

  # These resources will be updated later and should not 
  # be overridden.
  lifecycle {
    ignore_changes = [
      identifier_uris,
      required_resource_access,
    ]
  }

  feature_tags {
    enterprise            = true
    custom_single_sign_on = true
  }
}

# Configure a service principal.
resource "azuread_service_principal" "app_sp" {
  client_id = azuread_application.app.client_id

  # Sign-on URL.
  login_url                     = local.teleport_saml_entity_id
  preferred_single_sign_on_mode = "saml"

  feature_tags {
    enterprise            = true
    custom_single_sign_on = true
  }
}

# Configure a certificate to be used for SAML assertion signing.
resource "azuread_service_principal_token_signing_certificate" "app_saml_cert" {
  service_principal_id = azuread_service_principal.app_sp.id
  display_name         = "CN=${var.app_name}-sso-cert"
  # Choose the expiry date carefully, expired certificate will break user authentication.
  end_date = var.certificate_expiry_date
}

resource "azuread_application_identifier_uri" "app_identifier_uri" {
  depends_on = [azuread_service_principal.app_sp]

  application_id = azuread_application.app.id

  # Configures SAML Entity ID.
  identifier_uri = local.teleport_saml_entity_id
}

# By default, the azuread provider configures NameID format of 
# emailAddress type. Teleport expects the format to be of unspecified 
# type. To change this value, create a new userprincipalname claim.
resource "azuread_claims_mapping_policy" "app_nameid" {
  definition = [
    jsonencode(
      {
        ClaimsMappingPolicy = {
          ClaimsSchema = [
            {
              ID            = "userprincipalname"
              SamlClaimType = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/nameidentifier"
              Source        = "user"
            },
          ]
          IncludeBasicClaimSet = "true"
          Version              = 1
        }
      }
    ),
  ]
  display_name = "app_nameid"
}

# Attach the claim to the service principal.
resource "azuread_service_principal_claims_mapping_policy_assignment" "app_nameid_policy_assignment" {
  claims_mapping_policy_id = azuread_claims_mapping_policy.app_nameid.id
  service_principal_id     = azuread_service_principal.app_sp.id
}
