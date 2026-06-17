resource "teleport_inference_model" "prereq" {
  version = "v1"
  metadata = {
    name = "another-dummy-model"
  }
  spec = {
    openai = {
      openai_model_id = "gpt-4"
    }
  }
}

resource "teleport_inference_policy" "test-policy" {
  version = "v1"
  metadata = {
    name = "test-policy"
  }
  spec = {
    kinds = ["ssh", "db"]
    model = "another-dummy-model"
  }

  depends_on = [teleport_inference_model.prereq]
}
