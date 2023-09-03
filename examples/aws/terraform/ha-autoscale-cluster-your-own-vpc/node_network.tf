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
  count                    = var.create_bastion == true ? 1 : 0
  description              = "Allow SSH access via bastion"
  type                     = "ingress"
  from_port                = 22
  to_port                  = 22
  protocol                 = "tcp"
  security_group_id        = aws_security_group.node.id
  source_security_group_id = aws_security_group.bastion.id
}

resource "aws_security_group_rule" "node_ingress_allow_ssh_proxy" {
  count                    = var.create_bastion == true ? 1 : 0
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
