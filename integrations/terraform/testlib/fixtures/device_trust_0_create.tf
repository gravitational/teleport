resource "teleport_trusted_device" "TESTDEVICE1" {
  version = "v1"
  spec = {
    asset_tag     = "TESTDEVICE1"
    os_type       = "macos"
    enroll_status = "enrolled"
  }
}
