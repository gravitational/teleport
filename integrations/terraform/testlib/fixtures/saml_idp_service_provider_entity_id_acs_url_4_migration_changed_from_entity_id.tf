resource "teleport_saml_idp_service_provider" "test" {
  version = "v1"
  metadata = {
    name = "test-entity-id-acs-url-migration-changed"
  }
  spec = {
    entity_id = "https://sp.example.com/entity-id-acs-url/migration/changed/from/metadata"
    acs_url   = "https://sp.example.com/entity-id-acs-url/migration/changed/from/acs"
  }
}
