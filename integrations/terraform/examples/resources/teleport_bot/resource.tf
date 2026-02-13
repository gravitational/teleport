# Teleport Machine ID Bot creation example

resource "teleport_bot" "example" {
  metadata = {
    name = "example"
  }

  spec = {
    roles = ["access"]
  }
}
