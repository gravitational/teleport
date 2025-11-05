resource "teleport_access_monitoring_rule" "test" {
  version = "v1"
  metadata = {
    name = "test"
  }
  spec = {
    subjects      = ["access_request"]
    condition     = "access_request.spec.roles.contains(\"your_role_name\")"
    desired_state = "reviewed"
    notification = {
      name       = "slack"
      recipients = ["your-slack-channel"]
    }
    automatic_review = {
      integration = "builtin"
      decision    = "APPROVED"
    }
    schedules = {
      default = {
        time = {
          timezone = "America/Los_Angeles"
          shifts = [
            {
              weekday : "Monday"
              start : "00:00"
              end : "23:59"
            },
            {
              weekday : "Tuesday"
              start : "00:00"
              end : "23:59"
            },
          ]
        }
      }
    }
  }
}
