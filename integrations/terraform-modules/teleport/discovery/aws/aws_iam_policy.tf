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

  aws_iam_policy_organization_account_enumeration_name_prefix = (
    var.aws_organization_iam_policies.account_enumeration.use_name_prefix
    ? "${var.aws_organization_iam_policies.account_enumeration.name}-"
    : null
  )
  aws_iam_policy_organization_account_enumeration_name = (
    var.aws_organization_iam_policies.account_enumeration.use_name_prefix
    ? null
    : var.aws_organization_iam_policies.account_enumeration.name
  )

  aws_iam_policy_organization_join_validation_name_prefix = (
    var.aws_organization_iam_policies.join_validation.use_name_prefix
    ? "${var.aws_organization_iam_policies.join_validation.name}-"
    : null
  )
  aws_iam_policy_organization_join_validation_name = (
    var.aws_organization_iam_policies.join_validation.use_name_prefix
    ? null
    : var.aws_organization_iam_policies.join_validation.name
  )

  organization_account_enumeration_actions = [
    "organizations:ListAccountsForParent",
    "organizations:ListChildren",
    "organizations:ListRoots",
  ]

  organization_join_validation_actions = [
    "organizations:DescribeAccount",
  ]

  uses_ec2             = contains(local.aws_matcher_types, "ec2")
  uses_eks             = contains(local.aws_matcher_types, "eks")
  uses_rds             = contains(local.aws_matcher_types, "rds")
  uses_wildcard_region = contains(local.aws_matcher_regions, "*")

  ec2_actions = [
    "ec2:DescribeInstances",
    "ssm:DescribeInstanceInformation",
    "ssm:GetCommandInvocation",
    "ssm:ListCommandInvocations",
    "ssm:SendCommand",
  ]

  eks_actions = [
    "eks:ListClusters",
    "eks:DescribeCluster",
    "eks:ListAccessEntries",
    "eks:DescribeAccessEntry",
    "eks:CreateAccessEntry",
    "eks:DeleteAccessEntry",
    "eks:AssociateAccessPolicy",
    "eks:TagResource",
    "eks:UpdateAccessEntry",
  ]

  rds_actions = [
    "rds:DescribeDBClusters",
    "rds:DescribeDBInstances",
  ]

  resource_discovery_policy_actions = concat(
    local.uses_wildcard_region ? ["account:ListRegions"] : [],
    local.uses_ec2 ? local.ec2_actions : [],
    local.uses_eks ? local.eks_actions : [],
    local.uses_rds ? local.rds_actions : [],
  )
}

data "aws_iam_policy_document" "teleport_discovery_service_single_account" {
  count = local.create ? 1 : 0

  statement {
    effect    = "Allow"
    actions   = local.resource_discovery_policy_actions
    resources = ["*"]
  }
}

resource "aws_iam_policy" "teleport_discovery_service" {
  count = local.create && local.single_account_deployment ? 1 : 0

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

data "aws_iam_policy_document" "teleport_organization_account_enumeration" {
  count = local.create && local.organization_deployment ? 1 : 0

  statement {
    effect    = "Allow"
    actions   = local.organization_account_enumeration_actions
    resources = ["*"]
  }

  statement {
    effect    = "Allow"
    actions   = ["sts:AssumeRole"]
    resources = ["arn:${local.aws_partition}:iam::*:role/${var.aws_child_account_iam_role_name}"]

    condition {
      test     = "StringEquals"
      variable = "aws:ResourceOrgID"
      values   = [local.aws_organization_id]
    }
  }
}

# Trust policy for the IAM role created in each member account that allows the discovery service in the management account to assume it.
data "aws_iam_policy_document" "allow_assume_role_for_child_accounts" {
  count = local.create && local.organization_deployment ? 1 : 0

  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "AWS"
      identifiers = [aws_iam_role.teleport_discovery_service[0].arn]
    }

    condition {
      test     = "StringEquals"
      variable = "aws:PrincipalOrgID"
      values   = [local.aws_organization_id]
    }
  }
}

resource "aws_iam_policy" "teleport_organization_account_enumeration" {
  count = local.create && local.organization_discovery_with_integration ? 1 : 0

  description = "AWS IAM policy that grants the permissions needed for Teleport Discovery Service to enumerate accounts in the AWS organization."
  name        = local.aws_iam_policy_organization_account_enumeration_name
  name_prefix = local.aws_iam_policy_organization_account_enumeration_name_prefix
  path        = "/"
  tags        = local.apply_aws_tags
  policy = coalesce(
    var.aws_organization_iam_policies.account_enumeration.document,
    data.aws_iam_policy_document.teleport_organization_account_enumeration[0].json,
  )
}

data "aws_iam_policy_document" "teleport_organization_join_validation" {
  count = local.create && local.organization_deployment ? 1 : 0

  # Allow describing accounts so the Auth Service can resolve the joining identity's organization and Organizational Units when validating IAM-method join attempts.
  statement {
    effect    = "Allow"
    actions   = local.organization_join_validation_actions
    resources = ["*"]
  }
}

# This policy is only created in organization discovery mode when using the OIDC integration.
# When discovering an organization without using an OIDC integration, the permissions must be manually added and accessible to the Auth Service as ambient credentials.
resource "aws_iam_policy" "teleport_organization_join_validation" {
  count = local.create && local.organization_discovery_with_integration ? 1 : 0

  description = "AWS IAM policy that grants the permissions needed for Teleport Auth Service to accept join attempts based on the identity's organization."
  name        = local.aws_iam_policy_organization_join_validation_name
  name_prefix = local.aws_iam_policy_organization_join_validation_name_prefix
  path        = "/"
  tags        = local.apply_aws_tags
  policy = coalesce(
    var.aws_organization_iam_policies.join_validation.document,
    data.aws_iam_policy_document.teleport_organization_join_validation[0].json,
  )
}

################################################################################
# AWS IAM policy attachment for Teleport Discovery Service
################################################################################

resource "aws_iam_role_policy_attachment" "teleport_discovery_service" {
  count = local.create && local.single_account_deployment ? 1 : 0

  policy_arn = one(aws_iam_policy.teleport_discovery_service[*].arn)
  # we already know the role name, but use expression reference to establish
  # dependency on the role's existence
  role = one(aws_iam_role.teleport_discovery_service[*].name)
}

resource "aws_iam_role_policy_attachment" "teleport_organization_account_enumeration" {
  count = local.create && local.organization_discovery_with_integration ? 1 : 0

  policy_arn = one(aws_iam_policy.teleport_organization_account_enumeration[*].arn)
  # we already know the role name, but use expression reference to establish
  # dependency on the role's existence
  role = one(aws_iam_role.teleport_discovery_service[*].name)
}

resource "aws_iam_role_policy_attachment" "teleport_organization_join_validation" {
  count = local.create && local.organization_discovery_with_integration ? 1 : 0

  policy_arn = one(aws_iam_policy.teleport_organization_join_validation[*].arn)
  # we already know the role name, but use expression reference to establish
  # dependency on the role's existence
  role = one(aws_iam_role.teleport_discovery_service[*].name)
}
