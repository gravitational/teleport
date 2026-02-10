################################################################################
# AWS IAM policy and attachment for Teleport Discovery Service
################################################################################

locals {
  aws_iam_policy_name_prefix = (
    var.aws_iam_policy_use_name_prefix
    ? "${var.aws_iam_policy_name}-"
    : null
  )
  aws_iam_policy_name = (
    var.aws_iam_policy_use_name_prefix
    ? null
    : var.aws_iam_policy_name
  )
}

data "aws_iam_policy_document" "teleport_discovery_service_single_account" {
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

data "aws_iam_policy_document" "teleport_discovery_service_organization" {
  # Allow listing accounts in the organization.
  statement {
    effect = "Allow"

    actions = [
      "organizations:ListAccountsForParent",
      "organizations:ListChildren",
      "organizations:ListRoots",
    ]

    resources = ["*"]
  }

  # Allow assuming the role created in member accounts.
  statement {
    effect = "Allow"

    actions = [
      "sts:AssumeRole",
    ]

    resources = ["*"]
  }

  # Allow the Auth Service to accept org join attempts
  statement {
    effect = "Allow"

    actions = [
      "organizations:DescribeAccount",
    ]

    resources = ["*"]
  }
}

resource "aws_iam_policy" "teleport_discovery_service" {
  count = local.create ? 1 : 0

  description = "AWS IAM policy that grants the permissions needed for Teleport to discover resources in AWS."
  name        = local.aws_iam_policy_name
  name_prefix = local.aws_iam_policy_name_prefix
  path        = "/"
  tags        = local.apply_aws_tags
  policy = coalesce(
    var.aws_iam_policy_document,
    (
      local.single_account_deployment ?
      data.aws_iam_policy_document.teleport_discovery_service_single_account.json :
      data.aws_iam_policy_document.teleport_discovery_service_organization.json
    )
  )
}

################################################################################
# AWS IAM policy attachment for Teleport Discovery Service
################################################################################

resource "aws_iam_role_policy_attachment" "teleport_discovery_service" {
  count = local.create ? 1 : 0

  policy_arn = one(aws_iam_policy.teleport_discovery_service[*].arn)
  # we already know the role name, but use expression reference to establish
  # dependency on the role's existence
  role = one(aws_iam_role.teleport_discovery_service[*].name)
}
