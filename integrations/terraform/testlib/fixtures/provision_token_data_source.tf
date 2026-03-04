data "teleport_provision_token" "test" {
  kind    = "token"
  version = "v2"
  metadata = {
    name = "test"
  }
  spec = {
    roles = []
  }
}
