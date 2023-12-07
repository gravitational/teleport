resource "random_string" "token" {
  count  = var.agent_count
  length = 32
}

resource "teleport_provision_token" "agent" {
  count = var.agent_count
  spec = {
    roles = [
      // Uncomment the roles that correspond to the Teleport services you plan
      // to run on your agent nodes. We recommend running the SSH Service at a
      // minimum to enable secure access to the nodes.
      // "App",
      // "Db",
      // "Discovery",
      // "Kube",
      "Node",
    ]
    name = random_string.token[count.index].result
  }
  metadata = {
    expires = timeadd(timestamp(), "1h")
  }
}
