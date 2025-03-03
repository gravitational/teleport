resource "random_string" "token" {
  count            = var.agent_count
  length           = 32
  override_special = "-.+"
}

resource "teleport_provision_token" "agent" {
  count   = var.agent_count
  version = "v2"
  spec = {
    roles = ["Node"]
  }
  metadata = {
    name    = random_string.token[count.index].result
    expires = timeadd(timestamp(), "1h")
  }
}
