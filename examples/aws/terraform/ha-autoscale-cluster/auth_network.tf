// Auth subnets are for authentication servers
resource "aws_route_table" "auth" {
  count  = length(local.azs)
  vpc_id = local.vpc_id

  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Route all outbound traffic through NAT gateway
// Auth servers do not have public IP address and are located
// in their own subnet restricted by security group rules.
resource "aws_route" "auth" {
  count                  = length(local.azs)
  route_table_id         = element(aws_route_table.auth.*.id, count.index)
  destination_cidr_block = var.auth_aws_route_dest_cidr_block
  nat_gateway_id         = element(local.nat_gateways, count.index)
  depends_on             = [aws_route_table.auth]
}

// This is a private subnet for auth servers.
resource "aws_subnet" "auth" {
  count             = length(local.azs)
  vpc_id            = local.vpc_id
  cidr_block        = cidrsubnet(var.vpc_cidr, 8, count.index)
  availability_zone = element(local.azs, count.index)
  tags = {
    TeleportCluster = var.cluster_name
  }
}

resource "aws_route_table_association" "auth" {
  count          = length(local.azs)
  subnet_id      = element(aws_subnet.auth.*.id, count.index)
  route_table_id = element(aws_route_table.auth.*.id, count.index)
}

// Security groups for auth servers only allow access to 3025 port from
// public subnets, and not the internet
resource "aws_security_group" "auth" {
  name        = "${var.cluster_name}-auth"
  description = "Security group for ${var.cluster_name}-auth"
  vpc_id      = local.vpc_id
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// SSH emergency access via bastion security groups
resource "aws_security_group_rule" "auth_ingress_allow_ssh" {
  description              = "SSH emergency access via bastion security groups"
  type                     = "ingress"
  from_port                = 22
  to_port                  = 22
  protocol                 = "tcp"
  security_group_id        = aws_security_group.auth.id
  source_security_group_id = aws_security_group.bastion.id
}

// Internal traffic within the security group is allowed.
resource "aws_security_group_rule" "auth_ingress_allow_internal_traffic" {
  description       = "Internal traffic within the security group is allowed"
  type              = "ingress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  self              = true
  security_group_id = aws_security_group.auth.id
}

// Allow traffic from public subnet to auth servers - this is to
// let proxies to talk to auth server API.
// This rule uses CIDR as opposed to security group ip because traffic coming from NLB
// (network load balancer from Amazon)
// is not marked with security group ID and rules using the security group ids do not work,
// so CIDR ranges are necessary.
resource "aws_security_group_rule" "auth_ingress_allow_cidr_traffic" {
  description       = "Allow traffic from public subnet to auth servers in order to allow proxies to talk to auth server API"
  type              = "ingress"
  from_port         = 3025
  to_port           = 3025
  protocol          = "tcp"
  cidr_blocks       = aws_subnet.public.*.cidr_block
  security_group_id = aws_security_group.auth.id
}

// Allow traffic from nodes to auth servers.
// Teleport nodes heartbeat presence to auth server.
// This rule uses CIDR as opposed to security group ip because traffic coming from NLB
// (network load balancer from Amazon)
// is not marked with security group ID and rules using the security group ids do not work,
// so CIDR ranges are necessary.
resource "aws_security_group_rule" "auth_ingress_allow_node_cidr_traffic" {
  description       = "Allow traffic from nodes to auth servers in order to allow Teleport nodes heartbeat presence to auth server"
  type              = "ingress"
  from_port         = 3025
  to_port           = 3025
  protocol          = "tcp"
  cidr_blocks       = aws_subnet.node.*.cidr_block
  security_group_id = aws_security_group.auth.id
}

// All egress traffic is allowed
// tfsec:ignore:aws-ec2-no-public-egress-sgr
resource "aws_security_group_rule" "auth_egress_allow_all_traffic" {
  description       = "Permit all egress traffic"
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = var.allowed_auth_egress_cidr_blocks
  security_group_id = aws_security_group.auth.id
}

// Network load balancer for auth server.
resource "aws_lb" "auth" {
  name               = "${var.cluster_name}-auth"
  internal           = true
  subnets            = aws_subnet.public.*.id
  load_balancer_type = "network"
  idle_timeout       = 3600

  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Target group is associated with auto scale group
resource "aws_lb_target_group" "auth" {
  name     = "${var.cluster_name}-auth"
  port     = 3025
  vpc_id   = aws_vpc.teleport.id
  protocol = "TCP"
  // required to allow the use of IP pinning
  proxy_protocol_v2 = true
}

// 3025 is the Auth servers API server listener.
resource "aws_lb_listener" "auth" {
  load_balancer_arn = aws_lb.auth.arn
  port              = "3025"
  protocol          = "TCP"

  default_action {
    target_group_arn = aws_lb_target_group.auth.arn
    type             = "forward"
  }
}
