resource "teleport_auth_preference" "test" {
  version = "v2"
  metadata = {
    labels = {
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    disconnect_expired_cert = false
  }
}
