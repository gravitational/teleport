resource "teleport_role" "test" {
  version = "v7"
  metadata = {
    name        = "test"
    description = ""
    expires     = "2032-12-12T00:00:00Z"
  }

  spec = {
    options = {
      forward_agent   = true
      max_session_ttl = "2h3m"
    }
    allow = {
      logins = ["known", "anonymous"]
      request = {
        roles = ["example"]
        claims_to_roles = [
          {
            claim = "example"
            value = "example"
            roles = ["example"]
          },
        ]
      }

      node_labels = {
        "example" = ["yes", "no"]
      }
    }
  }
}
