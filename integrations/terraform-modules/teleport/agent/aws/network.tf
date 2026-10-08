locals {
  use_provided_subnets = var.private_subnet_ids != null || var.public_subnet_ids != null
  discover_subnets     = var.create && !local.use_provided_subnets

  explicit_subnet_route_table_ids = merge({}, [
    for route_table_id, route_table in data.aws_route_table.this : {
      for association in route_table.associations :
      association.subnet_id => route_table_id
      if association.subnet_id != null && association.subnet_id != ""
    }
  ]...)

  effective_subnet_route_table_ids = local.discover_subnets ? {
    for subnet_id in data.aws_subnets.this[0].ids :
    subnet_id => lookup(
      local.explicit_subnet_route_table_ids,
      subnet_id,
      data.aws_vpc.this[0].main_route_table_id,
    )
  } : {}

  private_route_table_ids = toset([
    for route_table_id, route_table in data.aws_route_table.this : route_table_id
    if anytrue([
      for route in route_table.routes :
      route.cidr_block == "0.0.0.0/0" && length(route.nat_gateway_id) > 0
    ])
  ])

  public_route_table_ids = toset([
    for route_table_id, route_table in data.aws_route_table.this : route_table_id
    if anytrue([
      for route in route_table.routes :
      route.cidr_block == "0.0.0.0/0" && startswith(route.gateway_id, "igw-")
    ])
  ])

  discovered_private_subnet_ids = sort([
    for subnet_id, route_table_id in local.effective_subnet_route_table_ids : subnet_id
    if contains(local.private_route_table_ids, route_table_id)
  ])

  discovered_public_subnet_ids = sort([
    for subnet_id, route_table_id in local.effective_subnet_route_table_ids : subnet_id
    if contains(local.public_route_table_ids, route_table_id)
  ])

  discovered_intra_subnet_ids = sort([
    for subnet_id, route_table_id in local.effective_subnet_route_table_ids : subnet_id
    if !contains(local.private_route_table_ids, route_table_id) && !contains(local.public_route_table_ids, route_table_id)
  ])

  private_subnet_ids = local.use_provided_subnets ? (
    var.private_subnet_ids != null ? sort(var.private_subnet_ids) : []
  ) : local.discovered_private_subnet_ids
  public_subnet_ids = local.use_provided_subnets ? (
    var.public_subnet_ids != null ? sort(var.public_subnet_ids) : []
  ) : local.discovered_public_subnet_ids
  intra_subnet_ids = local.use_provided_subnets ? [] : local.discovered_intra_subnet_ids

  preferred_subnet_ids = length(local.private_subnet_ids) > 0 ? local.private_subnet_ids : local.public_subnet_ids
  ecs_service_subnets  = slice(local.preferred_subnet_ids, 0, min(length(local.preferred_subnet_ids), 16))
  assign_public_ip     = length(local.private_subnet_ids) == 0 && length(local.public_subnet_ids) > 0
  database_access_security_group_ids = concat(
    var.database_service.database_security_group_ids,
    aws_security_group.teleport_database_access[*].id,
  )
}

resource "aws_security_group" "teleport_database_access" {
  count = local.create_security_group ? 1 : 0

  description = "Shared security group for Teleport Database Service access in ${var.vpc_id}."
  name_prefix = "${var.ecs_service_name}-database-access-"
  region      = var.region
  tags        = var.apply_aws_tags
  vpc_id      = var.vpc_id
}

resource "aws_vpc_security_group_ingress_rule" "allow_all_from_teleport_agent" {
  count = local.create_security_group ? length(local.database_access_security_group_ids) : 0

  ip_protocol                  = "-1"
  region                       = var.region
  referenced_security_group_id = one(aws_security_group.teleport_agent[*].id)
  security_group_id            = local.database_access_security_group_ids[count.index]
}
