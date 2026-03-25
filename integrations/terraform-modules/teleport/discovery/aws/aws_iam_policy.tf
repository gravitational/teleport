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

  uses_ec2 = contains(local.aws_matcher_types, "ec2")
  uses_eks = contains(local.aws_matcher_types, "eks")

  ec2_actions = concat(
    contains(local.aws_matcher_regions, "*") ? ["account:ListRegions"] : [],
    [
      "ec2:DescribeInstances",
      "ssm:DescribeInstanceInformation",
      "ssm:GetCommandInvocation",
      "ssm:ListCommandInvocations",
      "ssm:SendCommand",
    ]
  )

  eks_read_actions = [
    "eks:ListClusters",
    "eks:DescribeCluster",
    "eks:ListAccessEntries",
    "eks:DescribeAccessEntry",
  ]
  eks_write_actions = [
    "eks:CreateAccessEntry",
    "eks:DeleteAccessEntry",
    "eks:AssociateAccessPolicy",
    "eks:TagResource",
    "eks:UpdateAccessEntry",
  ]
  # EKS access-entry mutations are only used on the non-integration fetch path.
  # With OIDC enabled, this module sets matcher.integration and discovery stays
  # read-only for EKS.
  eks_actions = concat(
    local.eks_read_actions,
    local.use_oidc_integration ? [] : local.eks_write_actions
  )

  policy_actions = concat(
    local.uses_ec2 ? local.ec2_actions : [],
    local.uses_eks ? local.eks_actions : [],
  )
}

data "aws_iam_policy_document" "teleport_discovery_service_single_account" {
  count = local.create ? 1 : 0

  statement {
    effect    = "Allow"
    actions   = local.policy_actions
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
    data.aws_iam_policy_document.teleport_discovery_service_single_account[0].json,
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
