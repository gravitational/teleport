resource "teleport_saml_idp_service_provider" "from_descriptor" {
  version = "v1"
  metadata = {
    name = "my-sp"
  }
  spec = {
    entity_descriptor = <<-EOT
      <?xml version="1.0" encoding="UTF-8"?>
      <md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata"
          entityID="https://sp.example.com/saml/metadata">
        <md:SPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
          <md:AssertionConsumerService
              Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST"
              Location="https://sp.example.com/saml/acs"
              index="0"/>
        </md:SPSSODescriptor>
      </md:EntityDescriptor>
    EOT
    // If you set both entity_descriptor and attribute_mapping, the Teleport API
    // will add the attribute_mapping to the entity_descriptor, causing
    // the resource to change on every apply
    attribute_mapping = null
  }
}

resource "teleport_saml_idp_service_provider" "from_entity_id" {
  version = "v1"
  metadata = {
    name = "my-sp-2"
  }
  spec = {
    entity_id = "https://sp.example.com/saml/metadata"
    acs_url   = "https://sp.example.com/saml/acs"
    attribute_mapping = [
      {
        name = "username"
        // Note: the short forms (i.e. just "basic") are not supported by the
        // Terraform provider.
        name_format = "urn:oasis:names:tc:SAML:2.0:attrname-format:basic"
        value       = "external.username"
      },
    ]
  }
}