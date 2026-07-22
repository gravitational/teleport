# Teleport trusted cluster

resource "teleport_trusted_cluster" "cluster" {
  version = "v2"
  metadata = {
    name = "primary"
    labels = {
      test = "yes"
    }
  }

  spec = {
    enabled = false
    role_map = [{
      remote = "test"
      local  = ["admin"]
    }]
    proxy_addr = "localhost:3080"
    token      = "salami"
  }
}
