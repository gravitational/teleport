resource "teleport_scoped_role" "test" {
  version = "v1"
  metadata = {
    name = "test-scoped-role"
  }
  scope = "/staging"
  spec = {
    assignable_scopes = ["/staging/aa"]
    rules = [{
      resources = ["scoped_token"]
      verbs     = ["read", "list"]
    }]
  }
}
