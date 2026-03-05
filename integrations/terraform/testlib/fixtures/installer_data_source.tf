data "teleport_installer" "test" {
  version = "v1"
  metadata = {
    name = "test"
  }
  spec = {
    script = ""
  }
}
