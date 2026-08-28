resource "teleport_auth_preference" "cluster_auth_preference" {
  version = "v2"

  metadata = {
    labels = {
      provisioner           = "terraform"
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    name = "auth_preference"
    type = "oidc"
  }
}