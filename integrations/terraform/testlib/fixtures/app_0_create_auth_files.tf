resource "teleport_app" "test_auth_files" {
  version = "v3"
  metadata = {
    name        = "test_auth_files"
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
