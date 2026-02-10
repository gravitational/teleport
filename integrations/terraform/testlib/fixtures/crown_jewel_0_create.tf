resource "teleport_crown_jewel" "test" {
  version = "v1"
  metadata = {
    name        = "test"
    description = "Example crown jewel"
    labels = {
      foo = "bar"
    }
  }

  spec = {
    query = "resources.where(kind == \"db\")"
  }
}
