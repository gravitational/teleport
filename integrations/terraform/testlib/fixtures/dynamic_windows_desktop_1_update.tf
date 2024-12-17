resource "teleport_dynamic_windows_desktop" "test" {
  version = "v1"
  metadata = {
    name = "test"
    labels = {
      example               = "yes"
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    addr   = "localhost:3000"
    non_ad = false
    domain = "my.domain2"
  }
}
