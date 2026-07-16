data "aws_caller_identity" "this" {
  count = var.create ? 1 : 0
}

data "aws_region" "this" {
  count = var.create ? 1 : 0
}

data "aws_partition" "this" {
  count = var.create ? 1 : 0
}

data "aws_subnet" "teleport_agent" {
  count = var.create ? length(var.ecs_service_subnets) : 0

  id = var.ecs_service_subnets[count.index]
}

data "http" "managed_updates" {
  count = var.create && var.managed_updates_enabled ? 1 : 0

  url = format(
    "https://%s/webapi/find?group=%s",
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
