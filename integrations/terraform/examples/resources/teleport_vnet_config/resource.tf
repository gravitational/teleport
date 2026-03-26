# Teleport VNet config

resource "teleport_vnet_config" "example" {
  version = "v1"
  metadata = {
    description = "VNet config"
    labels = {
      "example"             = "yes"
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    ipv4_cidr_range = "100.64.0.0/10"
    custom_dns_zones = [{
      suffix = "internal.example.com"
      }, {
      suffix = "corp.example.com"
    }]
  }
}
