resource "teleport_dynamic_windows_desktop" "example" {
  version = "v1"
  metadata = {
    name        = "example"
    description = "Test Windows desktop"
    labels = {
      "teleport.dev/origin" = "dynamic" // This label is added on Teleport side by default
    }
  }

  spec = {
    addr   = "some.host.com"
    non_ad = true
    domain = "my.domain"
    screen_size = {
      width  = 800
      height = 600
    }
  }
}
