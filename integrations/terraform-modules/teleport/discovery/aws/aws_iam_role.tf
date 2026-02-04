################################################################################
# AWS IAM role for Teleport Discovery Service
################################################################################

locals {
  aws_iam_role_name_prefix = (
    var.aws_iam_role_use_name_prefix
    ? "${var.aws_iam_role_name}-"
    : null
  )
  aws_iam_role_name = (
    var.aws_iam_role_use_name_prefix
    ? null
    : var.aws_iam_role_name
  )
  trust_roles = try({
    local.trust_role.role_arn = local.trust_role
  }, {})
}

data "aws_iam_policy_document" "teleport_discovery_service_iam_role_trust" {
  count = local.create ? 1 : 0

  dynamic "statement" {
    for_each = local.use_oidc_integration ? [1] : []
    iterator = trust

    content {
      principals {
        type        = "Federated"
        identifiers = [local.aws_iam_oidc_provider_arn]
      }

      actions = [
        "sts:AssumeRoleWithWebIdentity"
      ]

      condition {
        test     = "StringEquals"
        variable = "${local.teleport_cluster_name}:aud"
        values   = [local.aws_iam_oidc_provider_aud]
      }
    }
  }

  dynamic "statement" {
    for_each = local.trust_roles
    iterator = trust

    content {
      actions = ["sts:AssumeRole"]

      principals {
        type        = "AWS"
        identifiers = trust.value.role_arn
      }

      condition {
        test     = "StringEquals"
        variable = "sts:ExternalId"
        values   = [trust.value.external_id]
      }
    }
  }
}

resource "aws_iam_role" "teleport_discovery_service" {
  count = local.create ? 1 : 0

  assume_role_policy   = data.aws_iam_policy_document.teleport_discovery_service_iam_role_trust[0].json
  description          = "AWS IAM role that Teleport Discovery Service will assume."
  max_session_duration = 3600
  name                 = local.aws_iam_role_name
  name_prefix          = local.aws_iam_role_name_prefix
  tags                 = local.apply_aws_tags
}

data "aws_iam_policy_document" "allow_assume_role_for_child_accounts" {
  statement {
    principals {
      type        = "AWS"
      identifiers = [aws_iam_role.teleport_discovery_service[0].arn]
    }

    actions = [
      "sts:AssumeRole"
    ]
  }
}
