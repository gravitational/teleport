locals {
  create = var.create
  default_tags = {
    "teleport.dev/cluster"     = local.teleport_cluster_name
    "teleport.dev/integration" = local.teleport_integration_name
    # this is the origin we set for resources created by the AWS OIDC integration web UI wizard.
    "teleport.dev/iac-tool" = "terraform"
  }
  apply_aws_tags                 = merge(local.default_tags, var.apply_aws_tags)
  apply_teleport_resource_labels = merge(local.default_tags, var.apply_teleport_resource_labels)

  aws_account_id = try(data.aws_caller_identity.this[0].account_id, "")
  aws_partition  = try(data.aws_partition.this[0].partition, "")

  teleport_cluster_name         = try(local.teleport_ping.cluster_name, "")
  teleport_ping                 = try(jsondecode(data.http.teleport_ping[0].response_body), null)
  teleport_proxy_public_addr    = var.teleport_proxy_public_addr
  teleport_proxy_public_url     = "https://${local.teleport_proxy_public_addr}"
  teleport_resource_name_suffix = "aws-account-${local.aws_account_id}"
}

data "aws_caller_identity" "this" {
  count = local.create ? 1 : 0
}

data "aws_partition" "this" {
  count = local.create ? 1 : 0
}

data "http" "teleport_ping" {
  count = local.create ? 1 : 0

  url = "${local.teleport_proxy_public_url}/webapi/find"

  request_headers = {
    Accept = "application/json"
  }
}
