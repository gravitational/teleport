locals {
  scope = "/staging"
}

resource "teleport_scoped_role_assignment" "test" {
  version = "v1"
  metadata = {
    name = "test-scoped-role-assignment"
  }
  scope    = local.scope
  sub_kind = "dynamic"
  spec = {
    user = "testuser"
    assignments = [{
      role  = "${local.scope}::test-scoped-role"
      scope = "${local.scope}/aa"
    }]
  }
}
