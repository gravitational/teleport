// Node subnets are for teleport nodes joining the cluster
// Nodes are not accessible via internet and are accessed
// via emergency access bastions or proxies
resource "aws_route_table" "node" {
  count  = length(local.azs)
  vpc_id = local.vpc_id

  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Route all outbound traffic through NAT gateway
resource "aws_route" "node" {
  count                  = length(local.azs)
  route_table_id         = element(aws_route_table.node.*.id, count.index)
  destination_cidr_block = var.node_aws_route_dest_cidr_block
  nat_gateway_id         = element(local.nat_gateways, count.index)
  depends_on             = [aws_route_table.node]
}

resource "aws_subnet" "node" {
  count             = length(local.azs)
  vpc_id            = local.vpc_id
  cidr_block        = cidrsubnet(var.vpc_cidr, 6, count.index + 1)
  availability_zone = element(local.azs, count.index)
  tags = {
    TeleportCluster = var.cluster_name
  }
}

resource "aws_route_table_association" "node" {
  count          = length(local.azs)
  subnet_id      = element(aws_subnet.node.*.id, count.index)
  route_table_id = element(aws_route_table.node.*.id, count.index)
}

// Node security groups do not allow direct internet access
// and only allow traffic coming in from proxies or
// emergency access bastions
resource "aws_security_group" "node" {
  name        = "${var.cluster_name}-node"
  description = "SG for ${var.cluster_name}-node"
  vpc_id      = local.vpc_id
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// SSH access is allowed via bastions and proxies
resource "aws_security_group_rule" "node_ingress_allow_ssh_bastion" {
  description              = "Allow SSH access via bastion"
  type                     = "ingress"
  from_port                = 22
  to_port                  = 22
  protocol                 = "tcp"
  security_group_id        = aws_security_group.node.id
  source_security_group_id = aws_security_group.bastion.id
}

resource "aws_security_group_rule" "node_ingress_allow_ssh_proxy" {
  description              = "Allow SSH access via proxy"
  type                     = "ingress"
  from_port                = 3022
  to_port                  = 3022
  protocol                 = "tcp"
  security_group_id        = aws_security_group.node.id
  source_security_group_id = aws_security_group.proxy.id
}

// tfsec:ignore:aws-ec2-no-public-egress-sgr
resource "aws_security_group_rule" "node_egress_allow_all_traffic" {
  description       = "Allow all egress traffic"
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = var.allowed_node_egress_cidr_blocks
  security_group_id = aws_security_group.node.id
}
