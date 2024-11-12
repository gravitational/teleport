# Teleport session recording config

resource "teleport_session_recording_config" "example" {
  version = "v2"
  metadata = {
    description = "Session recording config"
    labels = {
      "example"             = "yes"
      "teleport.dev/origin" = "dynamic" // This label is added on Teleport side by default
    }
  }

  spec = {
    proxy_checks_host_keys = true
  }
}
