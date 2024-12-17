resource "teleport_access_monitoring_rule" "test" {
  version = "v1"
  metadata = {
    name = "test"
  }
  spec = {
    subjects  = ["access_request"]
    condition = "access_request.spec.roles.contains(\"your_other_role_name\")"
    notification = {
      name       = "slack"
      recipients = ["your-slack-channel", "your-second-slack-channel"]
    }
  }
}