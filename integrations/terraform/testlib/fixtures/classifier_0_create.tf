resource "teleport_classifier" "test-classifier" {
  version = "v1"
  metadata = {
    name = "test-classifier"
  }
  spec = {
    kinds    = ["ssh", "k8s"]
    filter   = "equals(resource.metadata.labels[\"env\"], \"prod\")"
    criteria = "The user ran a potentially destructive command."
    actions = {
      emit_audit_event = 1
    }
  }
}
