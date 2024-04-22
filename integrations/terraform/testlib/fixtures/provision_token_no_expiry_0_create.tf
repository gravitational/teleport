resource "teleport_provision_token" "test" {
  version = "v2"
  metadata = {
    name = "test"
    labels = {
      example = "yes"
    }
  }
  spec = {
    roles = ["Node", "Auth"]
  }
}
