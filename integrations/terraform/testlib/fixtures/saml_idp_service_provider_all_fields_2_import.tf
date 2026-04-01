resource "teleport_saml_idp_service_provider" "test" {
  version = "v1"
  metadata = {
    name = "test-all-fields-import"
  }
  spec = {
    entity_id   = "https://sp.example.com/all-fields/import/metadata"
    acs_url     = "https://sp.example.com/all-fields/import/acs"
    preset      = "unspecified"
    relay_state = "https://example.com/relay-state"
    attribute_mapping = [
      {
        name        = "username"
        name_format = "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"
        value       = "external.username"
      },
      {
        name        = "groups"
        name_format = "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"
        value       = "external.groups"
      },
    ]
    launch_urls = [
      "https://example.com/launch/1",
      "https://example.com/launch/2"
    ]
  }
}
