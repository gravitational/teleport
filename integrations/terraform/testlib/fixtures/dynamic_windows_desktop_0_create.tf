resource "teleport_dynamic_windows_desktop" "test" {
  version = "v1"
  metadata = {
    name = "example"
    labels = {
      example               = "yes"
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    addr   = "localhost:3000"
    non_ad = true
    domain = "my.domain"
  }
}
