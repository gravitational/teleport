resource "teleport_vnet_config" "test" {
  version = "v1"
  spec = {
    ipv4_cidr_range = "192.168.2.0/24"
    custom_dns_zones = [
      { suffix = "internal.example.com" },
    ]
  }
}
