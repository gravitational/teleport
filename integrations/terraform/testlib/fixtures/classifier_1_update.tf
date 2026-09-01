resource "teleport_classifier" "test-classifier" {
  version = "v1"
  metadata = {
    name = "test-classifier"
  }
  spec = {
    kinds    = ["ssh"]
    filter   = "equals(resource.metadata.labels[\"env\"], \"staging\")"
    criteria = "The user ran a potentially destructive command."
    disabled = true
    actions = {
      emit_audit_event = true
      flag_for_review  = true
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
      {
        name   = "unjustified"
        filter = "equals(user.metadata.name, \"alice\")"
        actions = {
          flag_for_review = true
        }
      },
    ]
  }
}
