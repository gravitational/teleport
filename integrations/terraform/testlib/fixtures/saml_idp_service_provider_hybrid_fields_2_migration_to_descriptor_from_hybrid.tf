resource "teleport_saml_idp_service_provider" "test" {
  version = "v1"
  metadata = {
    name = "test-hybrid-fields-migration-to-descriptor"
  }
  spec = {
    entity_id         = "https://sp.example.com/hybrid-fields/migration/to-descriptor/from/metadata"
    acs_url           = "https://sp.example.com/hybrid-fields/migration/to-descriptor/from/acs"
    entity_descriptor = <<-XML
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://sp.example.com/hybrid-fields/migration/to-descriptor/from/metadata">
  <md:SPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sp.example.com/hybrid-fields/migration/to-descriptor/from/acs" index="0"/>
  </md:SPSSODescriptor>
</md:EntityDescriptor>
XML
  }
}
