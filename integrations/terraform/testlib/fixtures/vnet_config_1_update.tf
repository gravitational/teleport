resource "teleport_vnet_config" "test" {
  version = "v1"
  kind    = "vnet_config"
  metadata = {
    labels = {
      "example"             = "no"
      "teleport.dev/origin" = "dynamic"
    }
  }

  spec = {
    ipv4_cidr_range = "100.64.0.0/11"
    custom_dns_zones = [{
      suffix = "updated.example.com"
      }, {
      suffix = "svc.example.com"
    }]
  }
}
