module "aws_discovery" {
  source = "../.."

  teleport_proxy_public_addr    = "example.teleport.sh:443"
  teleport_discovery_group_name = "cloud-discovery-group"

  # Discover EC2 instances and EKS clusters with separate matching rules.
  # Both types accept "*" to discover across all enabled regions. The module
  # adds account:ListRegions to the IAM policy automatically when "*" is used.
  aws_matchers = [
    {
      types   = ["ec2"]
      regions = ["*"]
      tags = {
        env = ["prod"]
      }
    },
    {
      types   = ["eks"]
      regions = ["*"]
      tags = {
        team = ["platform"]
      }
      # Teleport's Kubernetes App Discovery will automatically identify and enroll HTTP applications running inside a Kubernetes cluster.
      kube_app_discovery = true
    }
  ]

  # Apply the additional Teleport label "origin=example" to all Teleport resources created by this module
  apply_teleport_resource_labels = { origin = "example" }
  # Apply the additional AWS tag "origin=example" to all AWS resources created by this module
  apply_aws_tags = { origin = "example" }
}
