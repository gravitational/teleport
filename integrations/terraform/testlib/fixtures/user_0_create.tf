resource "teleport_user" "test" {
  version = "v2"
  metadata = {
    name    = "test"
    expires = "2035-10-12T07:20:50Z"
    labels = {
      example = "yes"
    }
  }

  spec = {
    roles = ["terraform-provider"]

    traits = {
      logins1 = ["example"]
      logins2 = ["example"]
    }

    oidc_identities = [{
      connector_id = "oidc"
      username     = "example"
    }]

    github_identities = [{
      connector_id = "github"
      username     = "example"
    }]

    saml_identities = [{
      connector_id = "saml"
      username     = "example"
    }]
  }
}
