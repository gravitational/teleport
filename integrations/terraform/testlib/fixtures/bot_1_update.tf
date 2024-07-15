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

resource "teleport_bot" "test" {
  name     = local.bot_name
  token_id = "bot-test"
  roles    = ["terraform"]

  depends_on = [
    teleport_provision_token.bot_test
  ]

  traits = {
    logins1 = ["example"]
    logins2 = ["example"]
  }
}
