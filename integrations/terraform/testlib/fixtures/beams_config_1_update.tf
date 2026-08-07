resource "teleport_beams_config" "test" {
  version = "v1"

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
