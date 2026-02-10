resource "teleport_crown_jewel" "test" {
  version = "v1"
  metadata = {
    name        = "test"
    description = "Updated example crown jewel"
    labels = {
      foo = "baz"
    }
  }

  spec = {
    query = "resources.where(kind == \"app\")"
  }
}
