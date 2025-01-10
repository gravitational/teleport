resource "teleport_auth_preference" "cluster_auth_preference" {
  version = "v2"

  spec = {
    name = "auth_preference"
    type = "oidc"
  }
}