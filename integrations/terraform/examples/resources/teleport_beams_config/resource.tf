# Beams config

resource "teleport_beams_config" "example" {
  version = "v1"

  metadata = {
    name = "beams-config"
  }

  spec = {
    llm = {
      anthropic = {
        app_name = "my-anthropic"
      }
      openai = {
        app_name = "my-openai"
      }
    }
  }
}
