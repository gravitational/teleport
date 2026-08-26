resource "teleport_scoped_token" "test" {
  version = "v1"
  metadata = {
    name = "test-scoped-token"
  }
  scope = "/staging/aa"
  spec = {
    assigned_scope = "/staging/aa/nodes"
    join_method    = "token"
    roles          = ["Node"]
    usage_mode     = "unlimited"
  }
}
