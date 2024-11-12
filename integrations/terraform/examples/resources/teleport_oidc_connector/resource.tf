# Teleport OIDC connector
# 
# Please note that the OIDC connector will work in Teleport Enterprise only.

variable "oidc_secret" {}

resource "teleport_oidc_connector" "example" {
  version = "v3"
  metadata = {
    name = "example"
    labels = {
      test = "yes"
    }
  }

  spec = {
    client_id     = "client"
    client_secret = var.oidc_secret

    claims_to_roles = [{
      claim = "test"
      roles = ["terraform"]
    }]

    redirect_url = ["https://example.com/redirect"]
  }
}
