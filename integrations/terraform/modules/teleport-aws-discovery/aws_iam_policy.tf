################################################################################
# AWS IAM policy and attachment for Teleport Discovery Service
################################################################################

locals {
  create_aws_iam_policy = local.create && var.create_aws_iam_policy
  aws_iam_policy_name = "${local.name_prefix}${coalesce(
    var.aws_iam_policy_name,
    local.default_aws_resource_name,
  )}"
}

data "aws_iam_policy_document" "teleport_discovery_service_single_account" {
  count = local.create_aws_iam_policy ? 1 : 0

  statement {
    effect = "Allow"

    actions = [
      "account:ListRegions",
      "ec2:DescribeInstances",
      "ssm:DescribeInstanceInformation",
      "ssm:GetCommandInvocation",
      "ssm:ListCommandInvocations",
      "ssm:SendCommand",
    ]

    resources = ["*"]
  }
}

resource "aws_iam_policy" "teleport_discovery_service" {
  count = local.create_aws_iam_policy ? 1 : 0

  description = "AWS IAM policy that grants the permissions needed for Teleport to discover resources in AWS."
  name        = local.aws_iam_policy_name
  path        = "/"
  tags        = local.apply_aws_tags
  policy      = data.aws_iam_policy_document.teleport_discovery_service_single_account[0].json
}

data "aws_iam_policy" "teleport_discovery_service" {
  count = local.create && !local.create_aws_iam_policy ? 1 : 0

  name = local.aws_iam_policy_name
}

################################################################################
# AWS IAM policy attachment for Teleport Discovery Service
################################################################################

locals {
  create_aws_iam_policy_attachment = local.create && (local.create_aws_iam_policy || var.create_aws_iam_policy_attachment)
  discovery_aws_iam_policy_arn = try(
    aws_iam_policy.teleport_discovery_service[0].arn,
    data.aws_iam_policy.teleport_discovery_service[0].arn,
    ""
  )
}

resource "aws_iam_role_policy_attachment" "teleport_discovery_service" {
  count = local.create_aws_iam_policy_attachment ? 1 : 0

  policy_arn = local.discovery_aws_iam_policy_arn
  # we already know the role name, but use expression reference to establish
  # dependency on the role's existence
  role = try(
    aws_iam_role.teleport_discovery_service[0].name,
    data.aws_iam_role.teleport_discovery_service[0].name,
    ""
  )
}
