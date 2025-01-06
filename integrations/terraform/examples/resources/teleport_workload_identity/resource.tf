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
            equals    = "noah"
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