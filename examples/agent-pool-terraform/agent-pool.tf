resource "random_string" "token" {
  count  = var.agent_count
  length = 32
}

resource "teleport_provision_token" "agent" {
  count = var.agent_count
  spec = {
    roles = [
      "Node",
      "App",
      "Db",
      "Kube",
    ]
    name = random_string.token[count.index].result
  }
}

resource "aws_instance" "teleport_agent" {
  count = var.agent_count
  # Amazon Linux 2023 64-bit x86
  ami           = "ami-04a0ae173da5807d3"
  instance_type = "t3.small"
  subnet_id     = var.subnet_id
  user_data = templatefile("./userdata", {
    token                 = teleport_provision_token.agent[count.index].id
    proxy_service_address = var.proxy_service_address
    teleport_version      = var.teleport_version
  })

  // The following two blocks adhere to security best practices.

  metadata_options {
    http_tokens = "required"
  }

  root_block_device {
    encrypted = true
  }
}
