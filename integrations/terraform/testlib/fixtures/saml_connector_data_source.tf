data "teleport_saml_connector" "test" {
  kind    = "saml"
  version = "v2"
  metadata = {
    name = "test_data_source"
  }

  spec = {
    attributes_to_roles = [{
      name  = "groups"
      roles = ["saml-data-source-role"]
      value = "okta-admin"
    }]

    acs                   = "https://example.com/v1/webapi/saml/acs"
    entity_descriptor_url = "%v"
  }
}
