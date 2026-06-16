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
      emit_audit_event = true
      flag_for_review  = true
    }
  }
}
