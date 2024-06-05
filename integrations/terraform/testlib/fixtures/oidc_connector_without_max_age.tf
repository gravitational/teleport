resource "teleport_oidc_connector" "test_max_age" {
  version = "v3"
  metadata = {
    name    = "test_max_age"
    expires = "2032-10-12T07:20:50Z"
  }

  spec = {
    client_id     = "client"
    client_secret = "value"

    claims_to_roles = [{
      claim = "test"
      roles = ["teleport"]
    }]

    redirect_url = ["https://example.com/redirect"]
  }
}
