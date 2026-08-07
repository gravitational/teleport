resource "teleport_saml_idp_service_provider" "test" {
  version = "v1"
  metadata = {
    name = "test-entity-descriptor-migration-same"
  }
  spec = {
    entity_id = "https://sp.example.com/entity-descriptor/migration/same/metadata"
    acs_url   = "https://sp.example.com/entity-descriptor/migration/same/acs"
  }
}
