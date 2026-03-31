resource "teleport_ui_config" "example" {
  version = "v1"
  metadata = {
    description = "UI config"
    labels = {
      "example"             = "yes"
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    scrollback_lines = 1000
    show_resources   = "requestable"
  }
}
