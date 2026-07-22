resource "teleport_auth_preference" "main" {
  version = "v2"
  metadata = {
    description = "Require authentication via the ${var.oidc_connector_name} connector"
  }

  spec = {
    connector_name   = teleport_oidc_connector.main.metadata.name
    type             = "oidc"
    allow_local_auth = true
    second_factor    = "webauthn"
    webauthn = {
      rp_id = var.teleport_domain
    }
  }
}
