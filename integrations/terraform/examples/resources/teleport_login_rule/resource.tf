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

  # Either traits_map or traits_expression must be provided, but not both.
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
  #   # This traits_expression is functionally equivalent to the traits_map above.
  #   traits_expression = <<EOF
  # dict(
  #   pair("logins", union(external.logins, external.username))
  #   pair("groups", external.groups))
  # EOF
}
