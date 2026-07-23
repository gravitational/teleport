resource "teleport_beams_config" "test" {
  version = "v1"
  metadata = {
    labels = {
      "teleport.dev/origin" = "dynamic"
    }
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
