resource "teleport_saml_idp_service_provider" "test" {
  version = "v1"
  metadata = {
    name = "test-attribute-mapping-migration-to-mapping"
  }
  spec = {
    entity_id = "https://sp.example.com/attribute-mapping/migration/to-mapping/to/metadata"
    acs_url   = "https://sp.example.com/attribute-mapping/migration/to-mapping/to/acs"
    attribute_mapping = [
      {
        name        = "groups"
        name_format = "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"
        value       = "external.groups"
      },
    ]
  }
}
