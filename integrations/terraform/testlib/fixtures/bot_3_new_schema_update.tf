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
  version = "v1"

  metadata = {
    name = local.bot_name
    labels = {
      "teleport.dev/origin" = "config-file"
      "team"                = "engineering"
    }
    expires = "2038-01-01T00:00:00Z"
  }

  spec = {
    max_session_ttl = "1h"
    roles           = ["terraform", "deployer"]
  }

  depends_on = [
    teleport_provision_token.bot_test
  ]
}
