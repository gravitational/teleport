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
  count         = var.cloud == "aws" ? var.agent_count : 0
  ami           = data.aws_ami.amazon_linux_2023.id
  instance_type = "t3.small"
  subnet_id     = var.subnet_id
  user_data = templatefile("./userdata", {
    token                 = teleport_provision_token.agent[count.index].metadata.name
    proxy_service_address = var.proxy_service_address
    teleport_edition      = var.teleport_edition
    teleport_version      = var.teleport_version
  })

  // The following two blocks adhere to security best practices.

  associate_public_ip_address = false
  monitoring                  = true

  metadata_options {
    http_endpoint = "enabled"
    http_tokens   = "required"
  }

  root_block_device {
    encrypted = true
  }
}
