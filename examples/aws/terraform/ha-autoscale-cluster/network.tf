// Public subnets and routing tables used for NAT gateways
// and load balancers.
resource "aws_route_table" "public" {
  count  = length(local.azs)
  vpc_id = local.vpc_id

  tags = {
    TeleportCluster = var.cluster_name
  }
}

resource "aws_route" "public_gateway" {
  count                  = length(local.azs)
  route_table_id         = element(aws_route_table.public.*.id, count.index)
  destination_cidr_block = var.internet_gateway_dest_cidr_block
  gateway_id             = local.internet_gateway_id
  depends_on             = [aws_route_table.public]
}

resource "aws_subnet" "public" {
  count             = length(local.azs)
  vpc_id            = local.vpc_id
  cidr_block        = cidrsubnet(var.vpc_cidr, 8, count.index + 2)
  availability_zone = element(local.azs, count.index)

  tags = {
    TeleportCluster = var.cluster_name
  }
}

resource "aws_route_table_association" "public" {
  count          = length(local.azs)
  subnet_id      = element(aws_subnet.public.*.id, count.index)
  route_table_id = element(aws_route_table.public.*.id, count.index)
}

