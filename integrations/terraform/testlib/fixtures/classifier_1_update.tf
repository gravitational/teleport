resource "teleport_classifier" "test-classifier" {
  version = "v1"
  metadata = {
    name = "test-classifier"
  }
  spec = {
    kinds    = ["ssh"]
    filter   = "equals(resource.metadata.labels[\"env\"], \"staging\")"
    criteria = "The user ran a potentially destructive command."
    actions = {
      emit_audit_event = 1
      flag_for_review  = 1
    }
  }
}
