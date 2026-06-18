resource "teleport_lock" "test" {
  version = "v2"
  metadata = {
    name        = "test"
    description = "Ongoing incident investigation."
  }

  spec = {
    expires = "2026-12-31T00:00:00Z"
    message = "example_message"
    target = {
      access_request  = "example_uuid"
      bot_instance_id = "example_bot_instance_id"
      device          = "example_device_id"
      join_token      = "example_join_token"
      linux_desktop   = "example_linux_desktop"
      login           = "example_login"
      mfa_device      = "example_uuid"
      role            = "example_role"
      server_id       = "example_server_id"
      user            = "eve"
      windows_desktop = "example_windows_desktop"
    }
  }
}
