# Trusted device resource

resource "teleport_trusted_device" "TESTDEVICE1" {
  spec = {
    asset_tag = "TESTDEVICE1"
    os_type   = "macos"
  }
}
