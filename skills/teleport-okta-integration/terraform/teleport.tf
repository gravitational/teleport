resource "teleport_saml_connector" "okta" {
  version = "v2"
  metadata = {
    name   = "okta"
    labels = { "teleport.dev/origin" = "okta" }
  }
  spec = {
    display = "okta"
    acs     = local.acs
    # The app's public metadata URL.
    entity_descriptor_url = "${var.okta_org_url}/app/${okta_app_saml.teleport.entity_key}/sso/saml/metadata"
    attributes_to_roles = [{
      name = "groups"
      # The group assigned to the SAML app (main.tf).
      value = var.sso_group
      # Built-in Okta role for synced users.
      roles = ["okta-requester"]
    }]
  }
}
