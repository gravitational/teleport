data "teleport_health_check_config" "test" {
  metadata = {
    name = "test"
  }
  version = "v1"
  spec = {
    match = {}
  }
}
