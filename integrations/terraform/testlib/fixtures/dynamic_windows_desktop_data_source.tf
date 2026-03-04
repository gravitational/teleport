data "teleport_dynamic_windows_desktop" "test" {
  kind    = "dynamic_windows_desktop"
  version = "v1"
  metadata = {
    name = "test"
  }
  spec = {
    addr = ""
  }
}
