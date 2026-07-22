resource "teleport_saml_connector" "main" {
  version = "v2"
  metadata = {
    name = var.saml_connector_name
  }

  spec = {
    attributes_to_roles = var.saml_attributes_to_roles

    acs               = var.saml_acs
    entity_descriptor = var.saml_entity_descriptor
  }
}
