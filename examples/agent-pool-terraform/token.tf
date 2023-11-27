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
      "Discovery"
    ]
    name = random_string.token[count.index].result
  }
  metadata = {
    expires = timeadd(timestamp(), "1h")
  }
}
