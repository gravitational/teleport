resource "teleport_saml_idp_service_provider" "test" {
  version = "v1"
  metadata = {
    name = "test-entity-descriptor-migration-same"
  }
  spec = {
    entity_descriptor = <<-XML
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://sp.example.com/entity-descriptor/migration/same/metadata">
  <SPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sp.example.com/entity-descriptor/migration/same/acs" index="0"/>
  </SPSSODescriptor>
</EntityDescriptor>
XML
  }
}
