resource "teleport_scoped_role_assignment" "test" {
  version = "v1"
  metadata = {
    name = "test-scoped-role-assignment"
  }
  scope    = "/staging"
  sub_kind = "dynamic"
  spec = {
    user = "testuser"
    assignments = [{
      role  = "test-scoped-role"
      scope = "/staging/aa"
    }]
  }
}
