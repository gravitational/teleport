// Public subnets and routing tables used for NAT gateways
// and load balancers.
resource "aws_route_table" "public" {
  for_each  = var.az_list

  vpc_id = local.vpc_id

  tags = {
    Name            = "teleport-public-${each.key}"
    TeleportCluster = var.cluster_name
  }
}

resource "aws_route" "public_gateway" {
  for_each               = aws_route_table.public

  route_table_id         = each.value.id
  destination_cidr_block = "0.0.0.0/0"
  gateway_id             = local.internet_gateway_id
  depends_on             = [aws_route_table.public]
}

resource "aws_subnet" "public" {
  for_each          = var.az_list

  vpc_id            = local.vpc_id
  cidr_block        = cidrsubnet(local.bastion_cidr, 4, var.az_number[substr(each.key, 9, 1)])
  availability_zone = each.key

  tags = {
    Name            = "teleport-public-${each.key}"
    TeleportCluster = var.cluster_name
  }
}

# Creates a list which we can use to pick one subnet of all those created
locals {
  public_subnet_ids = [for subnet in aws_subnet.public : subnet.id]
}

resource "aws_route_table_association" "public" {
  for_each       = var.az_list

  subnet_id      = aws_subnet.public[each.key].id
  route_table_id = aws_route_table.public[each.key].id
}
