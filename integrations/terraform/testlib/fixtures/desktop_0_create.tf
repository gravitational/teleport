resource "teleport_desktop" "test" {
  version = "v1"
  metadata = {
    name        = "example"
    labels = {
      example               = "yes"
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    addr = "localhost:3000"
  }
}
