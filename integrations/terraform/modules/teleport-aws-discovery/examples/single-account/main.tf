module "aws_discovery" {
  source = "../.."

  teleport_proxy_public_addr    = "example.teleport.sh:443"
  teleport_discovery_group_name = "cloud-discovery-group"

  match_aws_tags           = { "*" : ["*"] }
  match_aws_resource_types = ["ec2"]
  name_prefix              = "example-tf"
  apply_aws_tags = {
    origin = "example"
  }
}
