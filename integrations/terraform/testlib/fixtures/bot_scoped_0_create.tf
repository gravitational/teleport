locals {
  bot_name_scoped = "test-scoped-bot"
  scope_path      = "/test-scope"
}

resource "teleport_scoped_role" "scoped_operator" {
  version = "v1"
  metadata = {
    name        = "scoped-operator"
    description = "Manage scoped roles, tokens, and assignments in test scope"
  }
  scope = local.scope_path
  spec = {
    assignable_scopes = [local.scope_path]
    rules = [{
      resources = ["scoped_role", "scoped_token", "scoped_role_assignment"]
      verbs     = ["create", "readnosecrets", "list", "update", "delete"]
    }]
  }
}

resource "teleport_bot" "test_scoped" {
  version = "v1"

  metadata = {
    name = local.bot_name_scoped
  }

  spec = {
    roles = []
  }

  scope = local.scope_path
}

resource "teleport_scoped_role_assignment" "bot_assignment" {
  version  = "v1"
  sub_kind = "dynamic"
  metadata = {
    name = "test-bot-assignment"
  }
  scope = local.scope_path
  spec = {
    bot_name  = teleport_bot.test_scoped.metadata.name
    bot_scope = teleport_bot.test_scoped.scope
    assignments = [{
      role  = teleport_scoped_role.scoped_operator.metadata.name
      scope = local.scope_path
    }]
  }
}