# Teleport Cluster Networking config

resource "teleport_cluster_maintenance_config" "example" {
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
