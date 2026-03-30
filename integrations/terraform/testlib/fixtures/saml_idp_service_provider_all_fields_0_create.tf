resource "teleport_saml_idp_service_provider" "test" {
  version = "v1"
  metadata = {
    name = "test-all-fields-create-update"
  }
  spec = {
    entity_id   = "https://sp.example.com/all-fields/create/metadata"
    acs_url     = "https://sp.example.com/all-fields/create/acs"
    preset      = "unspecified"
    relay_state = "https://example.com/relay-state"
    attribute_mapping = [
      {
        name        = "username"
        name_format = "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"
        value       = "external.username"
      },
      {
        name        = "foobar"
        name_format = "urn:oasis:names:tc:SAML:2.0:attrname-format:unspecified"
        value       = "external.foobar"
      },
      {
        name        = "website"
        name_format = "urn:oasis:names:tc:SAML:2.0:attrname-format:uri"
        value       = "external.website"
      },
    ]
    launch_urls = [
      "https://example.com/launch/1",
      "https://example.com/launch/2"
    ]
  }
}
