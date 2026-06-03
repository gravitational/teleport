# Teleport Machine ID Bot creation example

locals {
  bot_name_scoped = "scope-bot-admin"
  scope_path      = "/test-scope"
}

# Create the bot role
resource "teleport_scoped_role" "scoped_admin" {
  version = "v1"
  metadata = {
    name        = "scoped-admin"
    description = "Manages scoped roles, tokens, and assignments in the test scope."
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

# Create the bot
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

# Assign the role to the bot
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
      role  = teleport_scoped_role.scoped_admin.metadata.name
      scope = local.scope_path
    }]
  }
}
