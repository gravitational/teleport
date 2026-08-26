data "teleport_role" "test" {
  kind    = "role"
  version = "v8"
  metadata = {
    name = "test"
  }
}
