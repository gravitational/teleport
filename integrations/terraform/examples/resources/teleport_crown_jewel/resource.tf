resource "teleport_crown_jewel" "example" {
  version = "v1"
  metadata = {
    name        = "example"
    description = "Example crown jewel"
    labels = {
      environment = "production"
      owner       = "security"
    }
  }

  spec = {
    query = "resources.where(kind == \"db\")"
  }
}
