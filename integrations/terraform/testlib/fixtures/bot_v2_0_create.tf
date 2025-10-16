locals {
  bot_name = "test"
}

resource "teleport_provision_token" "bot_test" {
  version = "v2"

  metadata = {
    expires = "2038-01-01T00:00:00Z"
    name    = "bot-test"
  }

  spec = {
    roles       = ["Bot"]
    bot_name    = local.bot_name
    join_method = "token"
  }
}

resource "teleport_bot_v2" "test" {
  version = "v1"

  metadata = {
    name = local.bot_name
  }

  spec = {
    roles = ["terraform"]
  }
}
