module "aws_discovery" {
  source = "../.."

  teleport_proxy_public_addr    = "example.teleport.sh:443"
  teleport_discovery_group_name = "cloud-discovery-group"

  # Enroll resources from all AWS Accounts in the Organization
  enroll_organization_accounts = true

  # Discover EC2 AWS resources 
  match_aws_resource_types = ["ec2"]
  # Apply the additional AWS tag "origin=example" to all AWS resources created by this module
  apply_aws_tags = { origin = "example" }
  # Apply the additional Teleport label "origin=example" to all Teleport resources created by this module
  apply_teleport_resource_labels = { origin = "example" }

  # Examples of existing AWS resource reuse:
  # AWS IAM OIDC provider for this Teleport cluster's public proxy address must already exist
  create_aws_iam_openid_connect_provider = false

  # Example of specifying the IAM role name to assume in child accounts
  aws_iam_role_name_for_child_accounts = "teleport-discovery-marco"
}
