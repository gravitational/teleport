locals {
  apply_aws_tags = merge(var.apply_aws_tags, {
    "teleport.dev/cluster"     = local.teleport_cluster_name
    "teleport.dev/integration" = local.teleport_integration_name
    # this is the origin we set for resources created by the AWS OIDC integration web UI wizard.
    "teleport.dev/origin" = "integration_awsoidc"
  })
  create = var.create
  name_prefix = (
    var.name_prefix != ""
    ? "${trimsuffix(var.name_prefix, "-")}-"
    : ""
  )
  teleport_ping = try(jsondecode(data.http.teleport_ping[0].response_body), null)
}

data "aws_caller_identity" "this" {
  count = local.create ? 1 : 0
}

data "http" "teleport_ping" {
  count = local.create ? 1 : 0

  url = "${local.teleport_proxy_public_url}/webapi/ping"

  request_headers = {
    Accept = "application/json"
  }
}
