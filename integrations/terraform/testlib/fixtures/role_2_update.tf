resource "teleport_role" "test" {
  version = "v7"
  metadata = {
    name        = "test"
    description = "Test role"
    expires     = "2032-12-12T00:00:00Z"
  }

  spec = {
    options = {}

    allow = {
      logins = ["anonymous"]
      request = {
        roles = ["example", "terraform"]
        claims_to_roles = [
          {
            claim = "example"
            value = "example"
            roles = ["example"]
          },
        ]
      }

      node_labels = {
        "example" = ["no"]
        "sample"  = ["yes", "no"]
      }
    }
  }
}
