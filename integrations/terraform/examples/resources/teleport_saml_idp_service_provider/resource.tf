# Teleport SAML IdP Service Provider
#
# Please note that the SAML IdP Service Provider will work in Teleport Enterprise only.

resource "teleport_saml_idp_service_provider" "example" {
  version = "v1"

  metadata = {
    name = "iamshowcase"
  }

  spec = {
    entity_id = "iamshowcase"
    acs_url   = "https://sptest.iamshowcase.com/acs"
  }
}
