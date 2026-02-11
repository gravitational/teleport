resource "teleport_role" "test" {
  version = "v8"
  metadata = {
    name    = "test"
    expires = "2032-12-12T00:00:00Z"
  }

  spec = {
    allow = {
      logins      = ["anonymous"]
      request     = {}
      node_labels = {}
    }
  }
}
