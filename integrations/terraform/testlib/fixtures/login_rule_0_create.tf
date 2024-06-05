resource "teleport_login_rule" "expression_rule" {
  metadata = {
    name = "expression_rule"
    labels = {
      "env" = "test"
    }
  }

  version           = "v1"
  priority          = 1
  traits_expression = "external"
}

resource "teleport_login_rule" "map_rule" {
  metadata = {
    name = "map_rule"
    labels = {
      "env" = "test"
    }
  }

  version  = "v1"
  priority = 2
  traits_map = {
    "logins" = {
      values = [
        "external.logins",
        "external.username",
      ]
    }
  }
}
