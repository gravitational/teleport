module "aws_discovery" {
  source = "../.."

  teleport_proxy_public_addr    = "example.teleport.sh:443"
  teleport_discovery_group_name = "cloud-discovery-group"

  match_aws_tags                 = { "*" : ["*"] }
  match_aws_resource_types       = ["ec2"]
  apply_aws_tags                 = { origin = "example" }
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
      "arn:aws:ssm:*:*:document/*"
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
