resource "teleport_user" "test" {
  version = "v2"
  metadata = {
    name    = "test"
    expires = "2035-10-12T07:20:52Z"
    labels = {
      example = "yes"
    }
  }

  spec = {
    roles = ["terraform"]

    traits = {
      logins2 = ["example"]
    }

    oidc_identities = [{
      connector_id = "oidc-2"
      username     = "example"
    }]
  }
}
