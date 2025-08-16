data "teleport_role" "test" {
  kind    = "role"
  version = "v7"
  metadata = {
    name = "test"
  }
}
