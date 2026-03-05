data "teleport_app" "test" {
  kind    = "app"
  version = "v3"
  metadata = {
    name = "test"
  }
}
