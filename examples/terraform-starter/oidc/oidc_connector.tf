resource "teleport_oidc_connector" "main" {
  version = "v3"
  metadata = {
    name = var.oidc_connector_name
  }

  spec = {
    client_id       = var.oidc_client_id
    client_secret   = var.oidc_secret
    claims_to_roles = var.oidc_claims_to_roles
    redirect_url    = [var.oidc_redirect_url]
  }
}
