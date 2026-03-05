data "teleport_database" "test" {
  kind    = "db"
  version = "v3"
  metadata = {
    name = "test"
  }
}
