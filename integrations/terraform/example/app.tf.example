# Teleport App

resource "teleport_app" "example" {
  metadata = {
    name = "example"
    description = "Test app"
    labels = {
        "teleport.dev/origin" = "dynamic" // This label is added on Teleport side by default
    }
  }

  spec = {
    uri = "localhost:3000"
  }
}