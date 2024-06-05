resource "teleport_trusted_device" "TESTDEVICE1" {
  version = "v1"
  spec = {
    asset_tag     = "TESTDEVICE1"
    os_type       = "macos"
    enroll_status = "not_enrolled"
  }
}

resource "teleport_trusted_device" "TESTDEVICE2" {
  version = "v1"
  spec = {
    asset_tag     = "TESTDEVICE2"
    os_type       = "linux"
    enroll_status = "not_enrolled"
  }
}
