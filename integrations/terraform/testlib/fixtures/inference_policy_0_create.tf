resource "teleport_inference_model" "prereq" {
  version = "v1"
  metadata = {
    name = "dummy-model"
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
    kinds  = ["ssh", "k8s"]
    model  = "dummy-model"
    filter = "equals(resource.metadata.labels[\"env\"], \"prod\")"
  }

  depends_on = [teleport_inference_model.prereq]
}
