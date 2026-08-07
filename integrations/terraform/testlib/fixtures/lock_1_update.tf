resource "teleport_lock" "test" {
  version = "v2"
  metadata = {
    name        = "test"
    description = "Ongoing incident investigation."
  }

  spec = {
    target = {
      user = "eve"
    }
  }
}
