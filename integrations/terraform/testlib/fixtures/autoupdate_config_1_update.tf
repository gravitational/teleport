resource "teleport_autoupdate_config" "test" {
  version = "v1"
  spec = {
    tools = {
      mode = "enabled"
    }
    agents = {
      mode                        = "suspended"
      strategy                    = "time-based"
      maintenance_window_duration = "45m"
      schedules = {
        regular = [
          # dev is updated at 4:00 UTC
          { name = "dev", days = ["Mon", "Tue", "Wed", "Thu"], start_hour : 4 },
          # staging is updated at 08:00 UTC
          { name = "staging", days = ["Mon", "Tue", "Wed", "Thu"], start_hour : 8 },
          # prod is updated at 14:00 UTC
          { name = "prod", days = ["Mon", "Tue", "Wed", "Thu"], start_hour : 14, },
        ]
      }
    }
  }
}
