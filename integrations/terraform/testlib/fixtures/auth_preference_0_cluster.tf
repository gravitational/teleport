resource "teleport_auth_preference" "cluster_auth_preference" {
  version = "v2"

  spec = {
    type = "oidc"
  }
}