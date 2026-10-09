data "aws_caller_identity" "this" {
  count = var.create ? 1 : 0
}

data "aws_region" "this" {
  count = var.create ? 1 : 0

  region = var.region
}

data "aws_partition" "this" {
  count = var.create ? 1 : 0
}

data "aws_vpc" "this" {
  count = local.discover_subnets ? 1 : 0

  id     = var.vpc_id
  region = var.region
}

data "aws_subnets" "this" {
  count = local.discover_subnets ? 1 : 0

  region = var.region

  filter {
    name   = "vpc-id"
    values = [var.vpc_id]
  }
}

data "aws_route_tables" "this" {
  count = local.discover_subnets ? 1 : 0

  region = var.region
  vpc_id = var.vpc_id
}

data "aws_route_table" "this" {
  for_each = local.discover_subnets ? toset(data.aws_route_tables.this[0].ids) : toset([])

  route_table_id = each.value
  region         = var.region
}

data "http" "managed_updates" {
  count = var.create && var.managed_updates_enabled ? 1 : 0

  url = format(
    "https://%v/webapi/find?group=%v",
    local.managed_updates_proxy_addr,
    urlencode(coalesce(var.managed_updates_group, "default")),
  )

  lifecycle {
    precondition {
      condition     = !var.managed_updates_enabled || local.managed_updates_proxy_addr != ""
      error_message = "Managed Updates require teleport.proxy_server in teleport_config."
    }

    postcondition {
      condition = self.status_code == 200 && can(
        regex(
          "^v?[0-9]+\\.[0-9]+\\.[0-9]+.*",
          trimspace(jsondecode(self.response_body).auto_update.agent_version),
        )
      )
      error_message = <<EOF
Managed Updates endpoint must return HTTP 200 and a valid Teleport version.
Ensure the cluster supports Managed Updates and the configured update group exists.
EOF
    }
  }
}
