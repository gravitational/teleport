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
      emit_audit_event = true
    }
    rules = [
      {
        name     = "backups"
        criteria = "The destroyed resource was a backup or snapshot."
        actions = {
          risk_level_floor = "critical"
          emit_audit_event = true
        }
      },
    ]
  }
}
