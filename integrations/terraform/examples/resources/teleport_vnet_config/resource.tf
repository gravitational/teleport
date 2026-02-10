resource "teleport_vnet_config" "example" {
  version = "v1"
  metadata = {
    description = "Example VNet config"
    labels = {
      example               = "yes"
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    ipv4_cidr_range = "192.168.1.0/24"
    custom_dns_zones = [
      {
        suffix = "example.com"
      },
      {
        suffix = "corp.example.com"
      },
    ]
  }
}
