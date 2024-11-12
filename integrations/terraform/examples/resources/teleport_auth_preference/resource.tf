# AuthPreference resource

resource "teleport_auth_preference" "example" {
  version = "v2"
  metadata = {
    description = "Auth preference"
    labels = {
      "example"             = "yes"
      "teleport.dev/origin" = "dynamic" // This label is added on Teleport side by default
    }
  }

  spec = {
    disconnect_expired_cert = true
  }
}
