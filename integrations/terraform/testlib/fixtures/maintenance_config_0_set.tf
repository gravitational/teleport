resource "teleport_cluster_maintenance_config" "test" {
  version = "v1"
  metadata = {
    description = "Maintenance config"
  }

  spec = {
    agent_upgrades = {
      utc_start_hour = 1
      weekdays       = ["monday"]
    }
  }
}
