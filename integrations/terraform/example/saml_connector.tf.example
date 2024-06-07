# Teleport SAML connector
# 
# Please note that SAML connector will work in Enterprise version only. Check the setup docs:
# https://goteleport.com/docs/enterprise/sso/okta/

resource "teleport_saml_connector" "example" {
  # This block will tell Terraform to never update private key from our side if a keys are managed 
  # from an outside of Terraform.

  # lifecycle {
  #   ignore_changes = [
  #     spec[0].signing_key_pair[0].cert,
  #     spec[0].signing_key_pair[0].private_key,
  #     spec[0].assertion_key_pair[0].cert,
  #     spec[0].assertion_key_pair[0].private_key,
  #   ]
  # }

  # This section tells Terraform that role example must be created before the SAML connector
  depends_on = [
    teleport_role.example
  ]

  metadata = {
    name = "example"
  }

  spec = {
    attributes_to_roles = [{
      name  = "groups"
      roles = ["example"]
      value = "okta-admin"
    },
    {
      name  = "groups"
      roles = ["example"]
      value = "okta-dev"
    }]

    acs               = "https://localhost:3025/v1/webapi/saml/acs"
    entity_descriptor = ""
  }
}