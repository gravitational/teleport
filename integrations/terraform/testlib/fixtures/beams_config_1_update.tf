resource "teleport_beams_config" "test" {
  version = "v1"

  metadata = {
    name = "beams-config"
  }

  spec = {
    llm = {
      anthropic = {
        app_name = "updated-anthropic"
      }
      openai = {
        app_name = "updated-openai"
      }
    }
  }
}
