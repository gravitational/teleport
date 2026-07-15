resource "teleport_saml_idp_service_provider" "test" {
  version = "v1"
  metadata = {
    name = "test-all-fields-create-update"
  }
  spec = {
    entity_id   = "https://sp.example.com/all-fields/update/metadata"
    acs_url     = "https://sp.example.com/all-fields/update/acs"
    preset      = "gcp-workforce"
    relay_state = "https://example.com/updated-relay-state"
    attribute_mapping = [
      {
        name        = "department"
        name_format = "urn:oasis:names:tc:SAML:2.0:attrname-format:uri"
        value       = "external.department"
      },
      {
        name        = "email"
        name_format = "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"
        value       = "external.email"
      },
    ]
    launch_urls = [
      "https://example.com/updated-launch/1",
      "https://example.com/updated-launch/2"
    ]
  }
}
