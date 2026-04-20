################################################################################
# Teleport cluster discovery config
################################################################################

locals {
  teleport_discovery_config_name = (
    var.teleport_discovery_config_use_name_prefix
    ? "${var.teleport_discovery_config_name}-${local.teleport_resource_name_suffix}"
    : var.teleport_discovery_config_name
  )

  legacy_aws_matchers = length(var.match_aws_resource_types) == 0 ? [] : [
    {
      types                = var.match_aws_resource_types
      regions              = var.match_aws_regions
      tags                 = var.match_aws_tags
      setup_access_for_arn = ""
      kube_app_discovery   = null
    }
  ]

  effective_aws_matchers = (
    length(var.aws_matchers) > 0
    ? var.aws_matchers
    : local.legacy_aws_matchers
  )

  aws_matchers = [
    for matcher in local.effective_aws_matchers : merge(
      {
        types   = matcher.types
        regions = matcher.regions
        tags    = try(matcher.tags, { "*" : ["*"] })
        assume_role = (
          var.discovery_service_iam_credential_source.trust_role != null
          ? {
            role_arn = one(aws_iam_role.teleport_discovery_service[*].arn)
            external_id = (
              var.discovery_service_iam_credential_source.trust_role.external_id != ""
              ? var.discovery_service_iam_credential_source.trust_role.external_id
              : null
            )
          }
          : null
        )
        integration = (
          var.discovery_service_iam_credential_source.use_oidc_integration
          ? try(teleport_integration.aws_oidc[0].metadata.name, local.teleport_integration_name)
          : null
        )
      },
      contains(matcher.types, "ec2") ? {
        install = {
          enroll_mode      = 1 # INSTALL_PARAM_ENROLL_MODE_SCRIPT
          install_teleport = true
          join_method      = "iam"
          join_token       = local.teleport_provision_token_name
          script_name      = "default-installer"
          sshd_config      = "/etc/ssh/sshd_config"
        }
        ssm = {
          document_name = "AWS-RunShellScript"
        }
      } : {},
      contains(matcher.types, "eks") && matcher.setup_access_for_arn != "" ? {
        setup_access_for_arn = matcher.setup_access_for_arn
      } : {},
      contains(matcher.types, "eks") ? {
        kube_app_discovery = matcher.kube_app_discovery == false ? null : matcher.kube_app_discovery
      } : {}
    )
  ]

  aws_matcher_types = distinct(flatten([
    for matcher in local.aws_matchers : matcher.types
  ]))
  aws_matcher_regions = distinct(flatten([
    for matcher in local.aws_matchers : matcher.regions
  ]))
}

resource "teleport_discovery_config" "aws" {
  count = local.create ? 1 : 0

  lifecycle {
    precondition {
      condition = (
        length(var.aws_matchers) > 0
        || length(var.match_aws_resource_types) > 0
      )
      error_message = "aws_matchers must be set to discover your resources."
    }
    precondition {
      condition = !(
        length(var.aws_matchers) > 0 &&
        length(var.match_aws_resource_types) > 0
      )
      error_message = "aws_matchers and the legacy match_aws_* variables cannot be used together. Merge the legacy match variables into aws_matchers."
    }
    precondition {
      condition = !anytrue([
        for matcher in local.effective_aws_matchers :
        matcher.setup_access_for_arn != "" && var.discovery_service_iam_credential_source.use_oidc_integration
      ])
      error_message = "setup_access_for_arn requires discovery_service_iam_credential_source.use_oidc_integration to be false. OIDC integration bypasses EKS access entry setup."
    }
    precondition {
      condition = !(
        var.teleport_discovery_group_name == "cloud-discovery-group"
        && !var.discovery_service_iam_credential_source.use_oidc_integration
      )
      error_message = "The Discovery Service running in a Teleport Cloud cluster must use OIDC integration credentials. Either set discovery_service_iam_credential_source.use_oidc_integration to true or set teleport_discovery_group_name to a discovery group that is not `cloud-discovery-group`."
    }
  }

  header = {
    version = "v1"
    metadata = {
      description = "Configure Teleport to discover AWS resources."
      labels      = local.apply_teleport_resource_labels
      name        = local.teleport_discovery_config_name
    }
  }

  spec = {
    discovery_group = var.teleport_discovery_group_name
    aws             = local.aws_matchers
  }
}
