################################################################################
# AWS IAM role for Teleport Discovery Service
################################################################################

locals {
  aws_iam_oidc_provider_arn = try(
    aws_iam_openid_connect_provider.teleport[0].arn,
    data.aws_iam_openid_connect_provider.teleport[0].arn,
    "",
  )
  aws_iam_role_name = "${local.name_prefix}${coalesce(
    var.aws_iam_role_name,
    local.default_aws_resource_name,
  )}"
  create_aws_iam_role   = local.create && var.create_aws_iam_role
  teleport_cluster_name = try(local.teleport_ping.cluster_name, "")
  trust_roles = try({
    local.trust_role.role_arn = local.trust_role
  }, {})
}

data "aws_iam_policy_document" "teleport_discovery_service_iam_role_trust" {
  count = local.create_aws_iam_role ? 1 : 0

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
  count = local.create_aws_iam_role ? 1 : 0

  assume_role_policy   = data.aws_iam_policy_document.teleport_discovery_service_iam_role_trust[0].json
  description          = "AWS IAM role that Teleport Discovery Service will assume."
  max_session_duration = 3600
  name                 = local.aws_iam_role_name
  tags                 = local.apply_aws_tags
}

data "aws_iam_role" "teleport_discovery_service" {
  count = local.create && !local.create_aws_iam_role ? 1 : 0

  name = local.aws_iam_role_name
}
