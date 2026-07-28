resource "teleport_role" "test" {
  version = "v7"
  metadata = {
    name        = "saml-test-role"
    description = "SAML connector test role"
    expires     = "2032-12-12T00:00:00Z"
  }

  spec = {
    options = {}
    allow   = {}
  }
}

resource "teleport_saml_connector" "test" {
  version = "v2"
  metadata = {
    name    = "test"
    expires = "2032-10-12T07:20:50Z"
    labels = {
      example = "yes"
    }
  }

  spec = {
    attributes_to_roles = [{
      name  = "groups"
      roles = ["saml-test-role"]
      value = "okta-admin"
    }]

    acs                   = "https://example.com/v1/webapi/saml/acs"
    entity_descriptor_url = "%v/app/exk4d7tmnz9DEaEw85d7/sso/saml/metadata"
  }
}
