data "teleport_crown_jewel" "test" {
  kind    = "crown_jewel"
  version = "v1"
  metadata = {
    name = "test"
  }
  spec = {
    query = "resources.where(kind == \"db\")"
  }
}
