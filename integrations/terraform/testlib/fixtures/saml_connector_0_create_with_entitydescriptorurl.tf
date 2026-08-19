resource "teleport_role" "admin" {
  version = "v7"
  metadata = {
    name        = "terraform-saml-admin"
    description = "Terraform SAML test role"
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
      roles = ["terraform-saml-admin"]
      value = "okta-admin"
    }]

    acs                   = "https://example.com/v1/webapi/saml/acs"
    entity_descriptor_url = "%v/app/exk4d7tmnz9DEaEw85d7/sso/saml/metadata"
  }
}

data "teleport_saml_connector" "test" {
  version = teleport_saml_connector.test.version
  metadata = {
    name = teleport_saml_connector.test.metadata.name
  }

  spec = {
    attributes_to_roles   = teleport_saml_connector.test.spec.attributes_to_roles
    acs                   = teleport_saml_connector.test.spec.acs
    entity_descriptor_url = teleport_saml_connector.test.spec.entity_descriptor_url
  }
}
