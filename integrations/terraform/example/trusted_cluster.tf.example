# Teleport trusted cluster
#
# https://goteleport.com/docs/setup/admin/trustedclusters/

resource "teleport_trusted_cluster" "cluster" {
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
      local = ["admin"]
    }]
    proxy_addr = "localhost:3080"
    token = "salami"
  }
}
