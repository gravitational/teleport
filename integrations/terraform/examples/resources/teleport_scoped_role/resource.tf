# Teleport Scoped Role resource

resource "teleport_scoped_role" "example" {
  version = "v1"
  metadata = {
    name        = "example-scoped-role"
    description = "An example scoped role for managing resources within /staging"

    labels = {
      env = "staging"
    }
  }

  scope = "/staging"

  spec = {
    assignable_scopes = ["/staging/aa", "/staging/bb"]
    rules = [{
      resources = ["scoped_token"]
      verbs     = ["read", "list"]
    }]
    ssh = {
      logins = ["root"]
    }
  }
}
