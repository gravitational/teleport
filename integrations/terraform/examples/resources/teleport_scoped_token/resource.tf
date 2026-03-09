# Teleport Scoped Token resource

# A scoped token with unlimited usage for provisioning nodes within a sub-scope.
resource "teleport_scoped_token" "example" {
  version = "v1"
  metadata = {
    name        = "example-node-token"
    description = "Token for provisioning nodes in the /prod/us-east scope"

    labels = {
      env = "production22"
    }
  }

  scope = "/prod"

  spec = {
    assigned_scope = "/prod/us-east"
    roles          = ["Node"]
    join_method    = "token"
    usage_mode     = "unlimited"
  }
}

# A single-use scoped token that can only provision one resource.
resource "teleport_scoped_token" "single_use" {
  version = "v1"
  metadata = {
    name    = "example-single-use-token"
    expires = "2026-12-31T00:00:00Z"
  }

  scope = "/staging"

  spec = {
    assigned_scope = "/staging/eu-west"
    roles          = ["Node"]
    join_method    = "token"
    usage_mode     = "single_use"
  }
}
