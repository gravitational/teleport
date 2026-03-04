data "teleport_app_auth_config" "test" {
  kind    = "app_auth_config"
  version = "v1"
  metadata = {
    name = "test"
  }
  spec = {}
}
