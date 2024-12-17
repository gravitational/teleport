data "aws_ami" "amazon_linux_2023" {
  most_recent = true

  filter {
    name   = "description"
    values = ["Amazon Linux 2023 AMI*"]
  }

  filter {
    name   = "architecture"
    values = ["x86_64"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }

  filter {
    name   = "owner-alias"
    values = ["amazon"]
  }
}

resource "aws_instance" "teleport_agent" {
  count                       = length(var.userdata_scripts)
  ami                         = data.aws_ami.amazon_linux_2023.id
  instance_type               = "t3.small"
  subnet_id                   = var.subnet_id
  user_data                   = var.userdata_scripts[count.index]
  associate_public_ip_address = var.insecure_direct_access

  // Adheres to security best practices
  monitoring = true

  metadata_options {
    http_endpoint = "enabled"
    http_tokens   = "required"
  }

  root_block_device {
    encrypted = true
  }
}
