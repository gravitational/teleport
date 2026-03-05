data "teleport_auth_preference" "test" {
  kind    = "auth_preference"
  version = "v2"
  metadata = {
    name = "test"
  }
  spec = {}
}
