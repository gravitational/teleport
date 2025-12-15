locals {
  create = var.create
  name_prefix = (
    var.name_prefix != ""
    ? "${trimsuffix(var.name_prefix, "-")}-"
    : ""
  )
  apply_aws_tags = merge(var.apply_aws_tags, {
    "teleport.dev/cluster"     = local.teleport_cluster_name
    "teleport.dev/integration" = local.teleport_integration_name
    # this is the origin we set for resources created by the AWS OIDC integration web UI wizard.
    "teleport.dev/origin" = "integration_awsoidc"
  })
  teleport_ping              = try(jsondecode(data.http.teleport_ping[0].response_body), null)
  teleport_cluster_name      = try(local.teleport_ping.cluster_name, "")
  teleport_proxy_public_addr = var.teleport_proxy_public_addr
  teleport_proxy_public_url  = "https://${local.teleport_proxy_public_addr}"
}

data "http" "teleport_ping" {
  count = local.create ? 1 : 0

  url = "${local.teleport_proxy_public_url}/webapi/ping"

  # Optional request headers
  request_headers = {
    Accept = "application/json"
  }
}
################################################################################
# AWS IAM OIDC Provider
################################################################################

locals {
  create_aws_iam_openid_connect_provider = (
    local.create
    && local.use_oidc_integration
    && var.create_aws_iam_openid_connect_provider
  )
  aws_iam_oidc_provider_aud = "discover.teleport"
  # strip the port since AWS OIDC provider doesn't support port in the url
  aws_iam_oidc_provider_url = replace(local.teleport_proxy_public_url, "/:[0-9]+.*/", "")
  default_aws_resource_name = "teleport-discovery"
  use_oidc_integration      = var.discovery_service_iam_credential_source.use_oidc_integration
}

data "tls_certificate" "teleport_proxy" {
  count = local.create_aws_iam_openid_connect_provider ? 1 : 0

  url = local.teleport_proxy_public_url
}

# Create an AWS OIDC Provider, so that the Teleport Discovery Service can use
# OIDC to assume the discovery AWS IAM role.
resource "aws_iam_openid_connect_provider" "teleport" {
  count = local.create_aws_iam_openid_connect_provider ? 1 : 0

  url             = local.aws_iam_oidc_provider_url
  client_id_list  = [local.aws_iam_oidc_provider_aud]
  thumbprint_list = [data.tls_certificate.teleport_proxy[0].certificates[0].sha1_fingerprint]
  tags            = local.apply_aws_tags
}

data "aws_iam_openid_connect_provider" "teleport" {
  count = local.create && !local.create_aws_iam_openid_connect_provider && local.use_oidc_integration ? 1 : 0

  url = local.aws_iam_oidc_provider_url
}

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

################################################################################
# AWS IAM policy for Teleport Discovery Service
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

################################################################################
# Teleport cluster resources
################################################################################

locals {
  aws_account_id                 = try(data.aws_caller_identity.this[0].account_id, "")
  default_teleport_resource_name = "aws-account-${local.aws_account_id}"
}

data "aws_caller_identity" "this" {
  count = local.create ? 1 : 0
}

################################################################################
# Teleport cluster provision token for AWS
################################################################################

locals {
  create_teleport_provision_token = local.create
  teleport_provision_token_name = "${local.name_prefix}${coalesce(
    var.teleport_provision_token_name,
    local.default_teleport_resource_name,
  )}"
  apply_teleport_resource_labels = var.apply_teleport_resource_labels
}

resource "teleport_provision_token" "aws_iam" {
  count = local.create_teleport_provision_token ? 1 : 0

  metadata = {
    name        = local.teleport_provision_token_name
    description = "Allow Teleport nodes to join the cluster using AWS IAM credentials."
    labels      = local.apply_teleport_resource_labels
  }
  spec = {
    allow = [{
      aws_account = local.aws_account_id
    }]
    join_method = "iam"
    roles       = ["Node"]
  }
  version = "v2"
}

################################################################################
# Teleport cluster integration
################################################################################

locals {
  create_teleport_integration = local.create && local.use_oidc_integration
  teleport_integration_name = "${local.name_prefix}${coalesce(
    var.teleport_integration_name,
    local.default_teleport_resource_name,
  )}"
  discovery_aws_iam_role_arn = try(
    aws_iam_role.teleport_discovery_service[0].arn,
    data.aws_iam_role.teleport_discovery_service[0].arn,
    ""
  )
}

resource "teleport_integration" "aws_oidc" {
  count = local.create_teleport_integration ? 1 : 0

  metadata = {
    name        = local.teleport_integration_name
    description = "AWS OIDC integration for AWS discovery."
    labels      = local.apply_teleport_resource_labels
  }
  spec = {
    aws_oidc = {
      role_arn = local.discovery_aws_iam_role_arn
    }
  }
  sub_kind = "aws-oidc"
  version  = "v1"
}

################################################################################
# Teleport cluster discovery config
################################################################################

locals {
  assume_role = try({
    role_arn    = var.discovery_service_iam_credential_source.trust_role.role_arn
    external_id = var.discovery_service_iam_credential_source.trust_role.external_id
  })
  create_teleport_discovery_config = local.create
  match_aws_regions                = var.match_aws_regions
  match_aws_tags                   = var.match_aws_tags
  match_aws_types                  = var.match_aws_resource_types
  teleport_discovery_config_name = "${local.name_prefix}${coalesce(
    var.teleport_discovery_config_name,
    local.default_teleport_resource_name,
  )}"
  teleport_discovery_group_name = var.teleport_discovery_group_name
}

resource "teleport_discovery_config" "aws" {
  count = local.create_teleport_discovery_config ? 1 : 0

  header = {
    version = "v1"
    metadata = {
      name        = local.teleport_discovery_config_name
      description = "Configure Teleport to discover AWS resources."
      labels      = local.apply_teleport_resource_labels
    }
  }

  spec = {
    discovery_group = local.teleport_discovery_group_name
    aws = [{
      assume_role = local.assume_role
      install = {
        enroll_mode      = 1 # INSTALL_PARAM_ENROLL_MODE_SCRIPT
        install_teleport = true
        join_method      = "iam"
        join_token       = local.teleport_provision_token_name
        script_name      = "default-installer"
        sshd_config      = "/etc/ssh/sshd_config"
      }
      integration = (
        local.use_oidc_integration
        ? try(teleport_integration.aws_oidc[0].metadata.name, local.teleport_integration_name)
        : ""
      )
      regions = local.match_aws_regions
      ssm = {
        document_name = "AWS-RunShellScript"
      }
      tags  = local.match_aws_tags
      types = local.match_aws_types
    }]
  }
}

