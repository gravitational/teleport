resource "teleport_app" "test" {
  version = "v3"
  metadata = {
    name        = "example"
    description = "Test app"
    labels = {
      example               = "yes"
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    uri = "localhost:3000"
  }
}
