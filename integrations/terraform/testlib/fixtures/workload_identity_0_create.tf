resource "teleport_workload_identity" "test" {
  version = "v1"
  metadata = {
    name = "test"
  }
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
      id = "/test"
    }
  }
}