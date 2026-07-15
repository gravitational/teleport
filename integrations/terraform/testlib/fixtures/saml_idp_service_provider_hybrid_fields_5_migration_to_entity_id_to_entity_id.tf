resource "teleport_saml_idp_service_provider" "test" {
  version = "v1"
  metadata = {
    name = "test-hybrid-fields-migration-to-entity-id"
  }
  spec = {
    entity_id = "https://sp.example.com/hybrid-fields/migration/to-entity-id/to/metadata"
    acs_url   = "https://sp.example.com/hybrid-fields/migration/to-entity-id/to/acs"
  }
}
