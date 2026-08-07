resource "teleport_beams_config" "test" {
  version = "v1"

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
