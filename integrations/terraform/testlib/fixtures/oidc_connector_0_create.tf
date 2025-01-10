resource "teleport_oidc_connector" "test" {
  version = "v3"
  metadata = {
    name    = "test"
    expires = "2032-10-12T07:20:50Z"
    labels = {
      example = "yes"
    }
  }

  spec = {
    client_id     = "client"
    client_secret = "value"

    claims_to_roles = [{
      claim = "test"
      roles = ["terraform"]
    }]

    redirect_url = ["https://example.com/redirect"]
  }
}
