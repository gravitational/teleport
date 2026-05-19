locals {
  create = var.create
  apply_aws_tags = merge(var.apply_aws_tags, {
    "teleport.dev/cluster"     = local.teleport_cluster_name
    "teleport.dev/integration" = local.teleport_integration_name
    "teleport.dev/iac-tool"    = "terraform"
  })
  apply_teleport_resource_labels = merge(var.apply_teleport_resource_labels, {
    "teleport.dev/iac-tool" = "terraform",
  })

  organization_deployment   = var.aws_organization_discovery != null
  single_account_deployment = !local.organization_deployment

  organization_discovery_with_integration    = local.organization_deployment && var.discovery_service_iam_credential_source.use_oidc_integration
  organization_discovery_without_integration = local.organization_deployment && !var.discovery_service_iam_credential_source.use_oidc_integration

  # Ensure we fail loudly if the organization id is not available (ie, missing permissions) when doing organization discovery.
  aws_organization_id = local.organization_deployment ? data.aws_organizations_organization.this[0].id : null

  aws_account_id = try(data.aws_caller_identity.this[0].account_id, "")
  aws_partition  = try(data.aws_partition.this[0].partition, "")

  teleport_cluster_name     = try(local.teleport_ping.cluster_name, "")
  teleport_ping             = try(jsondecode(data.http.teleport_ping[0].response_body), null)
  teleport_proxy_public_url = "https://${var.teleport_proxy_public_addr}"
  teleport_resource_name_suffix = (
    local.organization_deployment
    ? "aws-organization-${local.aws_organization_id}"
    : "aws-account-${local.aws_account_id}"
  )
}

data "aws_caller_identity" "this" {
  count = local.create ? 1 : 0
}

data "aws_partition" "this" {
  count = local.create ? 1 : 0
}

data "aws_organizations_organization" "this" {
  count = local.create && local.organization_deployment ? 1 : 0
}

data "http" "teleport_ping" {
  count = local.create ? 1 : 0

  url = "${local.teleport_proxy_public_url}/webapi/find"

  request_headers = {
    Accept = "application/json"
  }
}
