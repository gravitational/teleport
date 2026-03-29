# Azure EntraID SAML application resources

# Normalize proxy URL to always include https://
locals {
  proxy_url = startswith(var.proxy_public_addr, "https://") ? var.proxy_public_addr : "https://${var.proxy_public_addr}"
}

# Get current Azure AD configuration
data "azuread_client_config" "current" {}

# Create the Application Registration
resource "azuread_application" "teleport_saml" {
  display_name = "Teleport ${local.proxy_url}"

  api {
    mapped_claims_enabled          = true
    requested_access_token_version = 2
  }
  web {
    redirect_uris = [
      "${local.proxy_url}/v1/webapi/saml/acs/${var.auth_connector_name}"
    ]
  }
  group_membership_claims = ["SecurityGroup"]
  optional_claims {
    saml2_token {
      name = "groups"
    }
  }
  sign_in_audience = "AzureADMyOrg"
}

# Set the Identifier URI (Entity ID) for SAML
resource "azuread_application_identifier_uri" "teleport_saml" {
  application_id = azuread_application.teleport_saml.id
  identifier_uri = "${local.proxy_url}/v1/webapi/saml/acs/${var.auth_connector_name}"
}

# Create the Enterprise Application (Service Principal)
resource "azuread_service_principal" "teleport_saml" {
  client_id = azuread_application.teleport_saml.client_id

  preferred_single_sign_on_mode = "saml"
  app_role_assignment_required = false

  feature_tags {
    enterprise            = true
    custom_single_sign_on = true
  }
}

# Create a SAML signing certificate
resource "azuread_service_principal_token_signing_certificate" "teleport_saml" {
  service_principal_id = azuread_service_principal.teleport_saml.id
  display_name         = "CN=azure-sso"
  end_date             = timeadd(timestamp(), "8760h") # 1 year
}

# Set the preferred signing certificate
resource "azuread_service_principal_certificate" "teleport_saml" {
  service_principal_id = azuread_service_principal.teleport_saml.id
  type                 = "AsymmetricX509Cert"
  value                = azuread_service_principal_token_signing_certificate.teleport_saml.value
  end_date             = azuread_service_principal_token_signing_certificate.teleport_saml.end_date
}

# Create Claims Mapping Policy to set NameID to email (optional)
resource "azuread_claims_mapping_policy" "teleport_email_nameid" {
  display_name = "Teleport NameID Email Policy"
  definition = [
    jsonencode({
      ClaimsMappingPolicy = {
        Version              = 1
        IncludeBasicClaimSet = "true"
        ClaimsSchema = [
          {
            Source           = "user"
            ID               = "mail"
            SamlClaimType    = "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/nameidentifier"
            SamlNameIdFormat = "urn:oasis:names:tc:SAML:1.1:nameid-format:emailAddress"
          }
        ]
      }
    })
  ]
}

# Assign the claims mapping policy to the service principal
resource "azuread_service_principal_claims_mapping_policy_assignment" "teleport_saml" {
  claims_mapping_policy_id = azuread_claims_mapping_policy.teleport_email_nameid.id
  service_principal_id     = azuread_service_principal.teleport_saml.id
}


