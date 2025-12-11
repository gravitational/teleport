locals {
  create                = var.create
  discover_organization = false # TODO(gavin): impl org discovery
  name_prefix = (
    var.name_prefix != ""
    ? "${trimsuffix(var.name_prefix, "-")}-"
    : ""
  )
  tags = merge(var.tags, {
    "teleport.dev/cluster"     = local.teleport_cluster_name
    "teleport.dev/integration" = local.teleport_integration_name
    # this is the origin we set for resources created by the AWS OIDC integration web UI wizard.
    "teleport.dev/origin" = "integration_awsoidc"
  })
  teleport_cluster_name      = var.teleport_cluster_name
  teleport_proxy_public_addr = var.teleport_proxy_public_addr
  teleport_proxy_public_url  = "https://${local.teleport_proxy_public_addr}"
}

################################################################################
# AWS IAM OIDC Provider
################################################################################

locals {
  create_aws_oidc_provider  = local.create
  aws_iam_oidc_provider_aud = "discover.teleport"
  # strip the port since AWS OIDC provider doesn't support port in the url
  aws_iam_oidc_provider_url = replace(local.teleport_proxy_public_url, "/:[0-9]+.*/", "")
  default_aws_resource_name = "teleport-discovery"
}

data "tls_certificate" "teleport_proxy" {
  count = local.create_aws_oidc_provider ? 1 : 0

  url = local.teleport_proxy_public_url
}

# Create an AWS OIDC Provider, so that the Teleport Discovery Service can use
# OIDC to assume the discovery AWS IAM role.
resource "aws_iam_openid_connect_provider" "teleport" {
  count = local.create_aws_oidc_provider ? 1 : 0

  url             = local.aws_iam_oidc_provider_url
  client_id_list  = [local.aws_iam_oidc_provider_aud]
  thumbprint_list = [data.tls_certificate.teleport_proxy[0].certificates[0].sha1_fingerprint]
  tags            = local.tags
}

################################################################################
# AWS IAM role for Teleport Discovery Service
################################################################################

locals {
  aws_iam_oidc_provider_arn = try(
    aws_iam_openid_connect_provider.teleport[0].arn,
    "",
  )
  create_teleport_discovery_service_iam_role = local.create
  teleport_discovery_service_iam_role_name = "${local.name_prefix}${coalesce(
    var.teleport_discovery_service_iam_role_name,
    local.default_aws_resource_name,
  )}"
  teleport_discovery_service_iam_role_trust_policy = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Principal": {
                "Federated": "${local.aws_iam_oidc_provider_arn}"
            },
            "Action": "sts:AssumeRoleWithWebIdentity",
            "Condition": {
                "StringEquals": {
                    "${local.teleport_cluster_name}:aud": "${local.aws_iam_oidc_provider_aud}"
                }
            }
        }
    ]
}
EOF
}

resource "aws_iam_role" "teleport_discovery_service" {
  count = local.create_teleport_discovery_service_iam_role ? 1 : 0

  assume_role_policy   = local.teleport_discovery_service_iam_role_trust_policy
  description          = "AWS IAM role that Teleport Discovery Service will assume."
  max_session_duration = 3600
  name                 = local.teleport_discovery_service_iam_role_name
  tags                 = local.tags
}

################################################################################
# AWS IAM policy for Teleport Discovery Service
################################################################################

locals {
  create_teleport_discovery_service_iam_policy            = local.create
  create_teleport_discovery_service_iam_policy_attachment = local.create_teleport_discovery_service_iam_policy
  teleport_discovery_service_iam_policy_name = "${local.name_prefix}${coalesce(
    var.teleport_discovery_service_iam_policy_name,
    local.default_aws_resource_name,
  )}"
  teleport_discovery_service_single_account_iam_policy       = <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "account:ListRegions",
                "ec2:DescribeInstances",
                "ssm:DescribeInstanceInformation",
                "ssm:GetCommandInvocation",
                "ssm:ListCommandInvocations",
                "ssm:SendCommand"
            ],
            "Resource": [
                "*"
            ]
        }
    ]
}
EOF
  teleport_discovery_service_organization_account_iam_policy = "" # TODO(gavin): impl org discovery
}

resource "aws_iam_policy" "teleport_discovery_service" {
  count = local.create_teleport_discovery_service_iam_policy ? 1 : 0

  description = "AWS IAM policy that grants the permissions needed for Teleport to discover resources in AWS."
  name        = local.teleport_discovery_service_iam_policy_name
  path        = "/"
  policy = (
    local.discover_organization
    ? local.teleport_discovery_service_organization_account_iam_policy
    : local.teleport_discovery_service_single_account_iam_policy
  )
  tags = local.tags
}

resource "aws_iam_role_policy_attachment" "teleport_discovery_service" {
  count = local.create_teleport_discovery_service_iam_policy_attachment ? 1 : 0

  policy_arn = one(aws_iam_policy.teleport_discovery_service[*].arn)
  role       = one(aws_iam_role.teleport_discovery_service[*].name)
}

################################################################################
# Teleport cluster resources
################################################################################

locals {
  aws_account_id      = try(data.aws_caller_identity.this[0].account_id, "")
  aws_organization_id = "" #TODO(gavin): impl org discovery
  default_teleport_resource_name = (
    local.discover_organization
    ? "aws-org-${local.aws_organization_id}"
    : "aws-account-${local.aws_account_id}"
  )
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
  teleport_resource_labels = var.teleport_resource_labels
}

resource "teleport_provision_token" "aws_iam" {
  count = local.create_teleport_provision_token ? 1 : 0

  metadata = {
    name        = local.teleport_provision_token_name
    description = "Allow Teleport nodes to join the cluster using AWS IAM credentials."
    labels      = local.teleport_resource_labels
  }
  spec = {
    allow = [{
      # TODO(gavin): impl org discovery
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
  create_teleport_integration = local.create
  teleport_integration_name = "${local.name_prefix}${coalesce(
    var.teleport_integration_name,
    local.default_teleport_resource_name,
  )}"
}

resource "teleport_integration" "aws_oidc" {
  count = local.create_teleport_integration ? 1 : 0

  metadata = {
    name        = local.teleport_integration_name
    description = "AWS OIDC integration for AWS discovery."
    labels      = local.teleport_resource_labels
  }
  spec = {
    aws_oidc = {
      role_arn = one(aws_iam_role.teleport_discovery_service[*].arn),
    }
  }
  sub_kind = "aws-oidc"
  version  = "v1"
}

################################################################################
# Teleport cluster discovery config
################################################################################

locals {
  create_teleport_discovery_config_aws = local.create
  exclude_aws_organizational_units     = "" # TODO(gavin): impl org discovery
  include_aws_organizational_units     = "" # TODO(gavin): impl org discovery
  match_aws_regions                    = var.match_aws_regions
  match_aws_tags                       = var.match_aws_tags
  match_aws_types                      = ["ec2"]
  teleport_discovery_config_name = "${local.name_prefix}${coalesce(
    var.teleport_discovery_config_name,
    local.default_teleport_resource_name,
  )}"
  teleport_discovery_group_name = var.teleport_discovery_group_name
}

resource "teleport_discovery_config" "aws" {
  count = local.create_teleport_discovery_config_aws ? 1 : 0

  header = {
    version = "v1"
    metadata = {
      name        = local.teleport_discovery_config_name
      description = "Configure Teleport to discover AWS resources."
      labels      = local.teleport_resource_labels
    }
  }

  spec = {
    discovery_group = local.teleport_discovery_group_name
    aws = [{
      install = {
        enroll_mode      = 1 # INSTALL_PARAM_ENROLL_MODE_SCRIPT
        install_teleport = true
        join_method      = "iam"
        join_token       = local.teleport_provision_token_name
        script_name      = "default-installer"
        sshd_config      = "/etc/ssh/sshd_config"
      }
      regions = local.match_aws_regions
      ssm = {
        document_name = "AWS-RunShellScript"
      }
      integration = one(teleport_integration.aws_oidc[*].metadata.name)
      # TODO(gavin): impl org discovery
      # organization = {
      #   organization_id = local.aws_organization_id
      #   organizational_units = {
      #     include = local.include_aws_organizational_units
      #     exclude = local.exclude_aws_organizational_units
      #   }
      # }
      tags  = local.match_aws_tags
      types = local.match_aws_types
    }]
  }
}
