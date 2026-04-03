module "aws_discovery" {
  source = "../.."

  teleport_proxy_public_addr    = "example.teleport.sh:443"
  teleport_discovery_group_name = "cloud-discovery-group"

  # Discover EC2 instances and EKS clusters with separate matching rules
  aws_matchers = [
    {
      types = ["ec2"]
      # EC2 discovery supports a wildcard to find instances in all regions.
      regions = ["*"]
      tags = {
        env = ["prod"]
      }
    },
    {
      types = ["eks"]
      # EKS requires region selection.
      regions = ["us-east-1"]
      tags = {
        team = ["platform"]
      }
    }
  ]

  # Apply the additional Teleport label "origin=example" to all Teleport resources created by this module
  apply_teleport_resource_labels = { origin = "example" }
  # Apply the additional AWS tag "origin=example" to all AWS resources created by this module
  apply_aws_tags = { origin = "example" }
}
