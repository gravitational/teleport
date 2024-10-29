resource "teleport_desktop" "test" {
  version = "v1"
  metadata = {
    name        = "test"
    labels = {
      example               = "yes"
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    addr = "localhost:3000"
  }
}
