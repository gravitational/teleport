module "aws_discovery" {
  source = "../.."

  teleport_proxy_public_addr    = "example.teleport.sh:443"
  teleport_discovery_group_name = "cloud-discovery-group"

  # Discover EC2 AWS resources 
  match_aws_resource_types = ["ec2"]
  # Apply the additional AWS tag "origin=example" to all AWS resources created by this module
  apply_aws_tags = { origin = "example" }
  # Apply the additional Teleport label "origin=example" to all Teleport resources created by this module
  apply_teleport_resource_labels = { origin = "example" }

  # Examples of existing AWS resource reuse:
  # AWS IAM OIDC provider for this Teleport cluster's public proxy address must already exist
  create_aws_iam_openid_connect_provider = false

  # Use a custom IAM policy with permission conditions
  aws_iam_policy_document = data.aws_iam_policy_document.teleport_discovery_service_single_account.json
}

data "aws_iam_policy_document" "teleport_discovery_service_single_account" {
  # read-only discovery
  statement {
    effect = "Allow"
    actions = [
      "account:ListRegions",
      "ec2:DescribeInstances",
      "ssm:DescribeInstanceInformation",
      "ssm:GetCommandInvocation",
      "ssm:ListCommandInvocations",
    ]
    resources = ["*"]
  }

  # SSM command execution document access
  # (Allows using any document, or restrict to specific documents here)
  statement {
    effect = "Allow"
    actions = [
      "ssm:SendCommand",
    ]
    resources = [
      "arn:aws:ssm:*:*:document/AWS-RunShellScript"
    ]
  }

  # SSM command execution instance access
  statement {
    effect = "Allow"
    actions = [
      "ssm:SendCommand",
    ]
    resources = [
      "arn:aws:ec2:*:*:instance/*"
    ]

    # restrict command execution on instances based on resource tags
    condition {
      test     = "StringEquals"
      variable = "aws:ResourceTag/TeleportManaged"
      values   = ["true"]
    }
  }
}
