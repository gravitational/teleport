resource "teleport_ui_config" "test" {
  version = "v1"
  metadata = {
    description = "Test UI config"
    labels = {
      "example"             = "yes"
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    scrollback_lines = 2000
    show_resources   = "requestable"
  }
}
