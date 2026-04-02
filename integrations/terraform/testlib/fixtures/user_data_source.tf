data "teleport_user" "test" {
  kind    = "user"
  version = "v2"
  metadata = {
    name = "test"
  }
}
