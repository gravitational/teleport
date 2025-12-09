module "aws_discovery" {
  source = "../.."

  teleport_cluster_name         = "example.teleport.sh"
  teleport_proxy_public_addr    = "example.teleport.sh:443"
  teleport_discovery_group_name = "cloud-discovery-group"

  create         = true
  match_aws_tags = { "*" : ["*"] }
  name_prefix    = "example-tf"
  tags = {
    origin = "example"
  }
}
