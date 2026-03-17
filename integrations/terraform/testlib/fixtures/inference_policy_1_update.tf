resource "teleport_inference_policy" "test-policy" {
  version = "v1"
  metadata = {
    name = "test-policy"
  }
  spec = {
    kinds = ["ssh", "db"]
    model = "another-dummy-model"
  }
}
