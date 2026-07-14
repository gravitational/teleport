# Teleport Machine ID Bot creation example

resource "teleport_bot" "example" {
  metadata = {
    name = "example"
    labels = {
      "teleport.dev/origin" = "dynamic" // This label is added on Teleport side by default
    }
  }

  spec = {
    roles = ["access"]
  }
}
