################################################################################
# Teleport cluster discovery config
################################################################################

locals {
  teleport_discovery_config_name = (
    var.teleport_discovery_config_use_name_prefix
    ? "${var.teleport_discovery_config_name}-${local.teleport_resource_name_suffix}"
    : var.teleport_discovery_config_name
  )
  trust_role = try({
    role_name   = (local.organization_deployment ? var.aws_iam_role_name_for_child_accounts : null)
    role_arn    = (local.single_account_deployment ? var.discovery_service_iam_credential_source.trust_role.role_arn : null)
    external_id = try(var.discovery_service_iam_credential_source.trust_role.external_id, null)
  }, null)
  organization = local.organization_deployment ? {
    organization_id = local.aws_organization_id
    organizational_units = {
      include = ["*"]
    }
  } : null
}

resource "teleport_discovery_config" "aws" {
  count = local.create ? 1 : 0

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
    aws = [{
      assume_role  = local.trust_role
      organization = local.organization
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
      regions = var.match_aws_regions
      ssm = {
        document_name = "AWS-RunShellScript"
      }
      tags  = var.match_aws_tags
      types = var.match_aws_resource_types
    }]
  }
}
