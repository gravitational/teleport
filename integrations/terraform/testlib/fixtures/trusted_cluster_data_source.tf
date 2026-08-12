data "teleport_trusted_cluster" "test" {
  version = "v2"
  metadata = {
    name = "%s"
  }

  spec = {}
}
