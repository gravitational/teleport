/* 
Security Groups and Rules for Cluster.

Note: Please see the list of networking ports documentation for their usage. 
https://goteleport.com/docs/setup/reference/networking/#ports
*/

// Create a Security Group
resource "aws_security_group" "cluster" {
  name   = "${var.cluster_name}-cluster"
  vpc_id = data.aws_vpc.default.id

  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Permit inbound to SSH
resource "aws_security_group_rule" "cluster_ingress_ssh" {
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = var.allowed_ssh_ingress_cidr_blocks
  security_group_id = aws_security_group.cluster.id
}
// Permit inbound to Teleport Web interface
resource "aws_security_group_rule" "cluster_ingress_web" {
  type              = "ingress"
  from_port         = 3080
  to_port           = 3080
  protocol          = "tcp"
  cidr_blocks       = var.allowed_ingress_cidr_blocks
  security_group_id = aws_security_group.cluster.id
}
// Permit inbound to Teleport services
resource "aws_security_group_rule" "cluster_ingress_services" {
  type              = "ingress"
  from_port         = 3022
  to_port           = 3025
  protocol          = "tcp"
  cidr_blocks       = var.allowed_ingress_cidr_blocks
  security_group_id = aws_security_group.cluster.id
}
// Permit inbound to Teleport mysql services
resource "aws_security_group_rule" "cluster_ingress_mysql" {
  type              = "ingress"
  from_port         = 3036
  to_port           = 3036
  protocol          = "tcp"
  cidr_blocks       = var.allowed_ingress_cidr_blocks
  security_group_id = aws_security_group.cluster.id
  count  = var.enable_mysql_listener ? 1 : 0
}

// Permit inbound to Teleport postgres services
resource "aws_security_group_rule" "cluster_ingress_postgres" {
  type              = "ingress"
  from_port         = 5432
  to_port           = 5432
  protocol          = "tcp"
  cidr_blocks       = var.allowed_ingress_cidr_blocks
  security_group_id = aws_security_group.cluster.id
  count  = var.enable_postgres_listener ? 1 : 0
}

// Permit inbound to Teleport mongodb services
resource "aws_security_group_rule" "cluster_ingress_mongodb" {
  type              = "ingress"
  from_port         = 27017
  to_port           = 27017
  protocol          = "tcp"
  cidr_blocks       = var.allowed_ingress_cidr_blocks
  security_group_id = aws_security_group.cluster.id
  count  = var.enable_mongodb_listener ? 1 : 0
}

// Permit all outbound traffic
resource "aws_security_group_rule" "cluster_egress" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = var.allowed_egress_cidr_blocks
  security_group_id = aws_security_group.cluster.id
}
