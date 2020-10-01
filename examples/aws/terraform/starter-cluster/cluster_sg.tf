/* 
Security Groups and Rules for Cluster.

Note: Please see our Production Guide for network security
recommendations. 
https://gravitational.com/teleport/docs/production/#firewall-configuration
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
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.cluster.id
}
// Permit inbound to Teleport Web interface
resource "aws_security_group_rule" "cluster_ingress_web" {
  type              = "ingress"
  from_port         = 3080
  to_port           = 3080
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.cluster.id
}
// Permit inbound to Teleport services
resource "aws_security_group_rule" "cluster_ingress_services" {
  type              = "ingress"
  from_port         = 3022
  to_port           = 3025
  protocol          = "tcp"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.cluster.id
}

// Permit all outbound traffic
resource "aws_security_group_rule" "cluster_egress" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = ["0.0.0.0/0"]
  security_group_id = aws_security_group.cluster.id
}