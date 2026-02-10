resource "teleport_vnet_config" "test" {
  version = "v1"
  spec = {
    ipv4_cidr_range = "10.10.0.0/16"
  }
}

data "teleport_vnet_config" "test" {
  version = "v1"
  kind    = "vnet_config"
  spec = {
    ipv4_cidr_range = "10.10.0.0/16"
  }
}
