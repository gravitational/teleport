# Teleport Cluster Networking config

resource "teleport_cluster_networking_config" "example" {
  version = "v2"
  metadata = {
    description = "Networking config"
    labels = {
      "example"             = "yes"
      "teleport.dev/origin" = "dynamic" // This label is added on Teleport side by default
    }
  }

  spec = {
    client_idle_timeout = "1h"
  }
}
