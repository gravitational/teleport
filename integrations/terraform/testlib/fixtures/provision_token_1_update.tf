resource "teleport_provision_token" "test" {
  version = "v2"
  metadata = {
    name    = "test"
    expires = "2038-01-01T00:00:00Z"
  }
  spec = {
    roles = ["Node"]
  }
}
