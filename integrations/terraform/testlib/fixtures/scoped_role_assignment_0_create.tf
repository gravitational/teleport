resource "teleport_scoped_role_assignment" "test" {
  version = "v1"
  metadata = {
    name = "dc6968fb-cbfb-4024-83f1-f6168310638b"
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
