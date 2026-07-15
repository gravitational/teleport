resource "teleport_saml_idp_service_provider" "test" {
  version = "v1"
  metadata = {
    name = "test-entity-id-acs-url-create-update"
  }
  spec = {
    entity_id = "https://sp.example.com/entity-id-acs-url/create/metadata"
    acs_url   = "https://sp.example.com/entity-id-acs-url/create/acs"
  }
}
