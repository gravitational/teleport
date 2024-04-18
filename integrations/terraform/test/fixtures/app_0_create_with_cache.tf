resource "teleport_app" "test_with_cache" {
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
