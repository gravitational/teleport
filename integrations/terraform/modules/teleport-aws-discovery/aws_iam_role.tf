################################################################################
# AWS IAM role for Teleport Discovery Service
################################################################################

locals {
  aws_iam_oidc_provider_arn = try(
    aws_iam_openid_connect_provider.teleport[0].arn,
    data.aws_iam_openid_connect_provider.teleport[0].arn,
    "",
  )
  create_aws_iam_role = local.create && var.create_aws_iam_role
  aws_iam_role_name = "${local.name_prefix}${coalesce(
    var.aws_iam_role_name,
    local.default_aws_resource_name,
  )}"
  trust_roles = ([
    for r in [
      var.discovery_service_iam_credential_source.trust_role,
    ] : r
    if r != null
  ])
  teleport_ping         = try(jsondecode(data.http.teleport_ping[0].response_body), null)
  teleport_cluster_name = try(local.teleport_ping.cluster_name, "")
}

data "http" "teleport_ping" {
  count = local.create ? 1 : 0

  url = "${local.teleport_proxy_public_url}/webapi/ping"

  # Optional request headers
  request_headers = {
    Accept = "application/json"
  }
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
