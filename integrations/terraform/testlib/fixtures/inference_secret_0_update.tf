resource "teleport_inference_secret" "test-secret" {
  version = "v1"
  metadata = {
    name = "test-secret"
  }
  spec = {
    value = "updated-api-key"
  }
}
