data "teleport_discovery_config" "test" {
  header = {
    metadata = {
      name = "test"
    }
    version = "v1"
  }
  spec = {
    discovery_group = ""
  }
}
