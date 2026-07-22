resource "teleport_saml_idp_service_provider" "test" {
  version = "v1"
  metadata = {
    name = "test-hybrid-fields-import"
  }
  spec = {
    entity_id         = "https://sp.example.com/hybrid-fields/import/metadata"
    acs_url           = "https://sp.example.com/hybrid-fields/import/acs"
    entity_descriptor = <<-XML
<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata" entityID="https://sp.example.com/hybrid-fields/import/metadata">
  <md:SPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <md:AssertionConsumerService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST" Location="https://sp.example.com/hybrid-fields/import/acs" index="0"/>
  </md:SPSSODescriptor>
</md:EntityDescriptor>
XML
  }
}
