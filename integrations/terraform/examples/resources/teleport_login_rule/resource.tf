# Teleport Login Rule resource

resource "teleport_login_rule" "example" {
  metadata = {
    description = "Example Login Rule"
    labels = {
      "example" = "yes"
    }
  }

  version  = "v1"
  priority = 0
  traits_map = {
    "logins" = {
      values = [
        "external.logins",
        "external.username",
      ]
    }
    "groups" = {
      values = [
        "external.groups",
      ]
    }
  }
}
