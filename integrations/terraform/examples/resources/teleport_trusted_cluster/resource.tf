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
    token          = "salami"
    tunnel_addr    = "rootcluster.example.com:443"
    web_proxy_addr = "rootcluster.example.com:443"
  }
}
