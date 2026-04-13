# Teleport Scoped Role Assignment resource
#
# Assigns an existing scoped role to a user at a specific scope.
# The referenced scoped role must already exist.

# resource "teleport_scoped_role" "example" {
#   version = "v1"
#   metadata = {
#     name = "example-scoped-role"
#   }

#   scope = "/staging"

#   spec = {
#     assignable_scopes = ["/staging/aa"]
#     rules = [{
#       resources = ["scoped_token"]
#       verbs     = ["read", "list"]
#     }]
#   }
# }

resource "teleport_scoped_role_assignment" "example" {
  version  = "v1"
  sub_kind = "dynamic"
  metadata = {
    name = "dc6961fa-cbfb-4024-83f1-f6168310638b"
  }

  scope = "/staging"

  spec = {
    user = "will"
    assignments = [{
      role  = "example-scoped-role"
      scope = "/staging/aa"
    }]
  }
}
