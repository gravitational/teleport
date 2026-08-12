resource "teleport_workload_identity" "test_scoped" {
  version = "v1"
  metadata = {
    name = "test-scoped"
  }
  scope = "/staging"
  spec = {
    rules = {
      allow = [
        {
          conditions = [{
            attribute = "user.name"
            eq = {
              value = "foo"
            }
          }]
        }
      ]
    }
    spiffe = {
      id = "/staging/_/test/updated"
    }
  }
}
