terraform {
  required_providers {
    teleport = {
      source  = "terraform.releases.teleport.dev/gravitational/teleport"
      version = "~> (=teleport.major_version=).0"
    }
  }
}

provider "teleport" {
  # Update addr to point to your Teleport proxy
  addr = "teleport.example.com:443"

  # Setting profile_dir and profile_name to empty strings will cause the
  # Terraform provider to authenticate using the current logged-in tsh profile
  profile_dir  = ""
  profile_name = ""
}

resource "teleport_login_rule" "terraform-test-map-rule" {
  metadata = {
    name        = "terraform-test-map-rule"
    description = "Terraform test rule using traits_map"
    labels = {
      example = "yes"
    }
  }
  version = "v1"

  # The rule with the lowest priority will be evaluated first.
  priority = 0

  # traits_map holds a map of all desired trait keys to list ofexpressions to
  # determine the trait values.
  traits_map = {

    # The "logins" traits will be set to the external "username" trait converted
    # to lowercase, as well as any external "logins" trait.
    "logins" = {

      # The traits_map value must be an object holding the expressions list in a
      # "values" field
      values = [
        "strings.lower(external.username)",
        "external.logins",
      ]
    }

    # The external "groups" trait will be passed through unchanged, all other
    # traits will be filtered out.
    "groups" = {
      values = [
        "external.groups",
      ]
    }
  }
}

resource "teleport_login_rule" "terraform-test-expression-rule" {
  metadata = {
    name        = "terraform-test-expression-rule"
    description = "Terraform test rule using traits_expression"
    labels = {
      example = "yes"
    }
  }
  version = "v1"

  # This rule has a higher priority value, so it will be evaluated after the
  # "terraform-test-map-rule".
  priority = 1

  # traits_expression is an alternative to traits_map, which returns all desired
  # traits in a single expression. The EOT syntax is a way of writing a
  # multiline string in Terraform, it is not part of the expression.
  traits_expression = <<-EOT
    external.put("groups",
      choose(
        option(external.groups.contains("admins"), external.groups.add("app-admins", "db-admins")),
        option(external.groups.contains("ops"), external.groups.add("k8s-admins")),
        option(true, external.groups)))
    EOT
}
