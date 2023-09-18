/*
Security Groups and Rules for Cluster.

Note: Please see the list of networking ports documentation for their usage.
https://goteleport.com/docs/setup/reference/networking/#ports
*/

// Create a Security Group
resource "aws_security_group" "cluster" {
  name        = "${var.cluster_name}-cluster"
  description = "${var.cluster_name} cluster"
  vpc_id      = data.aws_vpc.default.id

  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Permit inbound to SSH
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "cluster_ingress_ssh" {
  description       = "Permit inbound to SSH"
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = var.allowed_ssh_ingress_cidr_blocks
  security_group_id = aws_security_group.cluster.id
}

// Permit inbound to Teleport Web interface
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "cluster_ingress_web" {
  description       = "Permit inbound to Teleport web interface"
  type              = "ingress"
  from_port         = 443
  to_port           = 443
  protocol          = "tcp"
  cidr_blocks       = var.allowed_ingress_cidr_blocks
  security_group_id = aws_security_group.cluster.id
}

// Permit inbound to Teleport services
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "cluster_ingress_services" {
  description       = "Permit inbound to Teleport services"
  type              = "ingress"
  from_port         = 3022
  to_port           = 3026
  protocol          = "tcp"
  cidr_blocks       = var.allowed_ingress_cidr_blocks
  security_group_id = aws_security_group.cluster.id
  // don't expose other ports if ACM is enabled
  count = var.use_acm ? 0 : 1
}

// Permit inbound to Teleport mysql listener
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "cluster_ingress_mysql" {
  description       = "Permit inbound to Teleport mysql listener"
  type              = "ingress"
  from_port         = 3036
  to_port           = 3036
  protocol          = "tcp"
  cidr_blocks       = var.allowed_ingress_cidr_blocks
  security_group_id = aws_security_group.cluster.id
  // only expose if listener enabled and ACM disabled
  count = var.enable_mysql_listener ? !var.use_acm ? 1 : 0 : 0
}

// Permit inbound to Teleport postgres listener
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "cluster_ingress_postgres" {
  description       = "Permit inbound to Teleport postgres listener"
  type              = "ingress"
  from_port         = 5432
  to_port           = 5432
  protocol          = "tcp"
  cidr_blocks       = var.allowed_ingress_cidr_blocks
  security_group_id = aws_security_group.cluster.id
  // only expose if listener enabled and ACM disabled
  count = var.enable_postgres_listener ? !var.use_acm ? 1 : 0 : 0
}

// Permit inbound to Teleport mongodb listener
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "cluster_ingress_mongodb" {
  description       = "Permit inbound to Teleport mongodb listener"
  type              = "ingress"
  from_port         = 27017
  to_port           = 27017
  protocol          = "tcp"
  cidr_blocks       = var.allowed_ingress_cidr_blocks
  security_group_id = aws_security_group.cluster.id
  // only expose if listener enabled and ACM disabled
  count = var.enable_mongodb_listener ? !var.use_acm ? 1 : 0 : 0
}

// Permit all outbound traffic
// tfsec:ignore:aws-ec2-no-public-egress-sgr
resource "aws_security_group_rule" "cluster_egress" {
  description       = "Permit all outbound traffic"
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = var.allowed_egress_cidr_blocks
  security_group_id = aws_security_group.cluster.id
}
