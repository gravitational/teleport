resource "teleport_app" "test" {
  version = "v3"
  metadata = {
    name        = "test"
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
