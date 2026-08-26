resource "teleport_saml_idp_service_provider" "test" {
  version = "v1"
  metadata = {
    name = "test-attribute-mapping-import"
  }
  spec = {
    entity_id = "https://sp.example.com/attribute-mapping/import/metadata"
    acs_url   = "https://sp.example.com/attribute-mapping/import/acs"
    attribute_mapping = [
      {
        name        = "username"
        name_format = "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"
        value       = "external.username"
      },
    ]
  }
}
