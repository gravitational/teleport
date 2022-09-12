// Bastion is an emergency access bastion
// that could be spun up on demand in case
// of the need to have emergency administrative access
resource "aws_instance" "bastion" {
  count                       = "1"
  ami                         = data.aws_ami.base.id
  instance_type               = "t2.medium"
  key_name                    = var.key_name
  associate_public_ip_address = true
  source_dest_check           = false
  vpc_security_group_ids      = [aws_security_group.bastion.id]
  subnet_id                   = element(aws_subnet.public.*.id, 0)
  tags = {
    TeleportCluster = var.cluster_name
    TeleportRole    = "bastion"
  }
}

// Bastions are open to internet access
resource "aws_security_group" "bastion" {
  name   = "${var.cluster_name}-bastion"
  vpc_id = local.vpc_id
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Ingress traffic is allowed to SSH 22 port only
resource "aws_security_group_rule" "bastion_ingress_allow_ssh" {
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = var.allowed_bastion_ssh_ingress_cidr_blocks
  security_group_id = aws_security_group.bastion.id
}

// Egress traffic is allowed everywhere
resource "aws_security_group_rule" "proxy_egress_bastion_all_traffic" {
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = var.allowed_bastion_ssh_egress_cidr_blocks
  security_group_id = aws_security_group.bastion.id
}

