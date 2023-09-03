// Bastion is an emergency access bastion
// that could be spun up on demand in case
// of the need to have emergency administrative access
resource "aws_instance" "bastion" {
  count                       = var.create_bastion == true ? 1 : 0
  ami                         = data.aws_ami.base.id
  instance_type               = "t3.medium"
  key_name                    = var.key_name
  associate_public_ip_address = var.is_internal == true ? false : true
  source_dest_check           = false
  vpc_security_group_ids      = [aws_security_group.bastion.id]
  subnet_id                   = var.bastion_subnet

  metadata_options {
    http_endpoint = "enabled"
    http_tokens   = "required"
  }

  root_block_device {
    encrypted = true
  }

  tags = {
    TeleportCluster = var.cluster_name
    TeleportRole    = "bastion"
  }
}

// Bastions are open to internet access
resource "aws_security_group" "bastion" {
  name        = "${var.cluster_name}-bastion"
  description = "Bastions are open to internet access/or internally"
  vpc_id      = local.vpc_id
  tags = {
    TeleportCluster = var.cluster_name
  }
}

// Ingress traffic is allowed to SSH 22 port only
// tfsec:ignore:aws-ec2-no-public-ingress-sgr
resource "aws_security_group_rule" "bastion_ingress_allow_ssh" {
  description       = "Ingress traffic is allowed to SSH only"
  type              = "ingress"
  from_port         = 22
  to_port           = 22
  protocol          = "tcp"
  cidr_blocks       = var.allowed_bastion_ssh_ingress_cidr_blocks
  security_group_id = aws_security_group.bastion.id
  depends_on        = [aws_instance.bastion]
}

// Egress traffic is allowed everywhere
// tfsec:ignore:aws-ec2-no-public-egress-sgr
resource "aws_security_group_rule" "proxy_egress_bastion_all_traffic" {
  description       = "Egress traffic is allowed everywhere"
  type              = "egress"
  from_port         = 0
  to_port           = 0
  protocol          = "-1"
  cidr_blocks       = var.allowed_bastion_ssh_egress_cidr_blocks
  security_group_id = aws_security_group.bastion.id
  depends_on        = [aws_instance.bastion]
}
