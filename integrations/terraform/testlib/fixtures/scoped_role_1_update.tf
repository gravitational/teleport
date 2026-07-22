resource "teleport_scoped_role" "test" {
  version = "v1"
  metadata = {
    name = "test-scoped-role"
    labels = {
      env = "staging"
    }
  }
  scope = "/staging"
  spec = {
    assignable_scopes = ["/staging/aa", "/staging/bb"]
    rules = [{
      resources = ["scoped_token"]
      verbs     = ["read", "list", "create"]
    }]
    ssh = {
      logins = ["root"]
    }
  }
}
