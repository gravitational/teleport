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
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.auth.id
}

// Allow traffic from nodes to auth servers.
// Teleport nodes heartbeat presence to auth server.
// This rule uses CIDR as opposed to security group ip because traffic coming from NLB
// (network load balancer from Amazon)
// is not marked with security group ID and rules using the security group ids do not work,
// so CIDR ranges are necessary.
# resource "aws_security_group_rule" "auth_ingress_allow_node_cidr_traffic" {
#   description       = "Allow traffic from nodes to auth servers in order to allow Teleport nodes heartbeat presence to auth server"
#   type              = "ingress"
#   from_port         = 3025
#   to_port           = 3025
#   protocol          = "tcp"
#   cidr_blocks       = ["0.0.0.0/0"]
#   security_group_id = aws_security_group.auth.id
# }

// This rule allows non NLB traffic originating directly from proxies
resource "aws_security_group_rule" "auth_ingress_allow_public_traffic" {
  description              = "Allow non-NLB traffic originating directly from proxies"
  type                     = "ingress"
  from_port                = 3025
  to_port                  = 3025
  protocol                 = "tcp"
  source_security_group_id = aws_security_group.proxy.id
  security_group_id        = aws_security_group.auth.id
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
  subnets            = var.auth_subnets
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
  vpc_id   = local.vpc_id
  protocol = "TCP"
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
