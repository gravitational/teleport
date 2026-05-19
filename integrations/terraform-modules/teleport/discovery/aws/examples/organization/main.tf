module "aws_discovery" {
  source = "../.."

  teleport_proxy_public_addr    = "example.teleport.sh:443"
  teleport_discovery_group_name = "cloud-discovery-group"

  # Enroll resources from all AWS Accounts in the Organization
  # Only EC2 resource discovery is supported for organization-wide discovery.
  aws_organization_discovery = {
    organizational_units = {
      # Include accounts under any Organizational Unit.
      # At least one organizational unit must be included. You can use the root ID or the `*` to include the entire organization.
      include = ["*"]
      # Exclude the Organizational Unit's accounts and all their descendants.
      # Takes precedence over the include rule, so accounts under this OU will not be enrolled even if the include rule matches them.
      # Only exact matches are supported for exclusion, wildcards are not allowed.
      exclude = ["ou-1234-abcdwxyz"]
    }
  }

  # Each child account in the organization must have an IAM role with this name.
  # This role is assumed by the discovery service to enroll resources from that account.
  # The required trust relationship and permissions for this role can be found in the module outputs and documentation.
  aws_child_account_iam_role_name = "teleport-organization-discovery-child-account-role"

  # Discover EC2 instances with matching rules
  aws_matchers = [
    {
      types = ["ec2"]
      # EC2 discovery supports a wildcard to find instances in all regions.
      regions = ["*"]
      tags = {
        env = ["prod"]
      }
    }
  ]

  # Apply the additional Teleport label "origin=example" to all Teleport resources created by this module
  apply_teleport_resource_labels = { origin = "example" }
  # Apply the additional AWS tag "origin=example" to all AWS resources created by this module
  apply_aws_tags = { origin = "example" }
}