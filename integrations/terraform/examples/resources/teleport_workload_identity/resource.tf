resource "teleport_workload_identity" "example" {
  version = "v1"
  metadata = {
    name = "example"
  }
  spec = {
    rules = {
      allow = [
        {
          conditions = [{
            attribute = "user.name"
            eq = {
              value = "my-user"
            }
          }]
        }
      ]
    }
    spiffe = {
      id   = "/my/spiffe/id/path"
      hint = "my-hint"
    }
  }
}