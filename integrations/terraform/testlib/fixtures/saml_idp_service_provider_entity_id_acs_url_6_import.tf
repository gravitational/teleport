resource "teleport_saml_idp_service_provider" "test" {
  version = "v1"
  metadata = {
    name = "test-entity-id-acs-url-import"
  }
  spec = {
    entity_id = "https://sp.example.com/entity-id-acs-url/import/metadata"
    acs_url   = "https://sp.example.com/entity-id-acs-url/import/acs"
  }
}
