data "teleport_classifier" "test" {
  kind    = "classifier"
  version = "v1"
  metadata = {
    name = "test-classifier"
  }
  spec = {
    kinds    = ["ssh", "k8s"]
    criteria = "The user ran a potentially destructive command."
  }
}
