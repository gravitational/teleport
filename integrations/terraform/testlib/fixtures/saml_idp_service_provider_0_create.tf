resource "teleport_saml_idp_service_provider" "test" {
  version = "v1"
  metadata = {
    name = "iamshowcase"
    labels = {
      example = "yes"
    }
  }
  spec = {
    entity_id = "iamshowcase"
    acs_url   = "https://sptest.iamshowcase.com/acs"
  }
}
