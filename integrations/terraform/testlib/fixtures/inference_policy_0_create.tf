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
}
