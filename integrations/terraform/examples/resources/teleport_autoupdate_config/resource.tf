resource "teleport_autoupdate_config" "test" {
  version = "v1"
  spec = {
    tools = {
      mode = "enabled"
    }
    agents = {
      mode     = "enabled"
      strategy = "halt-on-error"
      schedules = {
        regular = [
          {
            name = "dev"
            days = ["Mon", "Tue", "Wed", "Thu"]
            start_hour : 4
          },
          {
            name = "staging"
            days = ["Mon", "Tue", "Wed", "Thu"]
            start_hour : 14
          },
          {
            name = "prod"
            days = ["Mon", "Tue", "Wed", "Thu"]
            start_hour : 14
            wait_hours : 24
          },
        ]
      }
    }
  }
}
