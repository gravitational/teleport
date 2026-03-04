data "teleport_static_host_user" "test" {
  version = "v2"
  metadata = {
    name = "test"
  }
  spec = {
    matchers = []
  }
}
